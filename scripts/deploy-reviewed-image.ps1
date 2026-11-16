param([string]$Server = 'root@201.18.206.109')

$ErrorActionPreference = 'Stop'
$releaseCommit = (git rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $releaseCommit -cnotmatch '^[a-f0-9]{40}$') { throw 'Cannot identify the release revision' }
if ((git branch --show-current).Trim() -ne 'main') { throw 'Deployments require main' }
git diff --quiet HEAD
if ($LASTEXITCODE -ne 0) { throw 'Commit the reviewed source before preparing a release image' }
$remoteCommit = (git ls-remote origin refs/heads/main).Split()[0]
if ($LASTEXITCODE -ne 0 -or $remoteCommit -ne $releaseCommit) { throw 'The release revision must be published on origin/main' }

$image = "vy2hpttvmehhauvskvilgoju:$releaseCommit"
$releaseFolder = Join-Path ([IO.Path]::GetTempPath()) "weevcrafts-release-$releaseCommit"
New-Item -ItemType Directory -Path $releaseFolder -Force | Out-Null
$contextFile = Join-Path $releaseFolder 'source.tar'
$imageFile = Join-Path $releaseFolder 'application.tar'
git archive --format=tar --output=$contextFile HEAD
if ($LASTEXITCODE -ne 0) { throw 'Cannot export the committed release source' }
$env:WEEVCRAFTS_IMAGE_CONTEXT = $contextFile
$env:WEEVCRAFTS_RELEASE_COMMIT = $releaseCommit
$env:WEEVCRAFTS_RELEASE_IMAGE = $image
try {
@'
import os,subprocess
with open(os.environ['WEEVCRAFTS_IMAGE_CONTEXT'],'rb') as source:
    subprocess.run(['docker','build','--platform','linux/amd64',
        '--build-arg','SOURCE_COMMIT='+os.environ['WEEVCRAFTS_RELEASE_COMMIT'],
        '--label','org.opencontainers.image.revision='+os.environ['WEEVCRAFTS_RELEASE_COMMIT'],
        '-t',os.environ['WEEVCRAFTS_RELEASE_IMAGE'],'-'],stdin=source,check=True)
'@ | python -
    if ($LASTEXITCODE -ne 0) { throw 'The release image build failed' }
} finally {
    Remove-Item Env:WEEVCRAFTS_IMAGE_CONTEXT,Env:WEEVCRAFTS_RELEASE_COMMIT,Env:WEEVCRAFTS_RELEASE_IMAGE -ErrorAction SilentlyContinue
}

$deadline = (Get-Date).AddMinutes(30)
do {
    $runs = gh run list --workflow 'Launch checks' --commit $releaseCommit --limit 1 --json databaseId,status,conclusion,headSha
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect the release checks' }
    $run = @($runs | ConvertFrom-Json)[0]
    if ($run -and $run.status -eq 'completed') {
        if ($run.conclusion -ne 'success' -or $run.headSha -ne $releaseCommit) { throw 'Launch checks failed for this release revision' }
        break
    }
    Write-Output "Waiting for Launch checks on $releaseCommit"
    Start-Sleep -Seconds 10
} while ((Get-Date) -lt $deadline)
if (-not $run -or $run.status -ne 'completed') { throw 'Release checks are still pending' }

docker image save --output $imageFile $image
if ($LASTEXITCODE -ne 0) { throw 'Cannot export the reviewed release image' }
$digest = (Get-FileHash -Algorithm SHA256 -LiteralPath $imageFile).Hash.ToLowerInvariant()
$remoteFile = "/root/.weevcrafts-integrations/application-$releaseCommit.tar"
scp -q -o BatchMode=yes $imageFile "${Server}:$remoteFile"
if ($LASTEXITCODE -ne 0) { throw 'Cannot transfer the reviewed release image' }
$remoteScript = @'
import hashlib,json,pathlib,re,subprocess
commit='__COMMIT__'
expected='__DIGEST__'
p=pathlib.Path('/root/.weevcrafts-integrations/application-'+commit+'.tar')
with p.open('rb') as f:
    actual=hashlib.file_digest(f,'sha256').hexdigest()
if actual!=expected:raise RuntimeError('Transferred image checksum mismatch')
subprocess.run(['docker','load','--input',str(p)],check=True)
image='vy2hpttvmehhauvskvilgoju:'+commit
data=json.loads(subprocess.check_output(['docker','image','inspect',image],text=True))[0]
if data['Config']['Labels'].get('org.opencontainers.image.revision')!=commit:
    raise RuntimeError('Loaded image revision mismatch')
php="""$a=App\Models\Application::where('uuid','vy2hpttvmehhauvskvilgoju')->firstOrFail();
if ($a->git_branch!=='main' || $a->build_pack!=='dockerfile' || $a->pendingDeploymentConfigurationDiff()->requiresBuild()) { throw new RuntimeException('Application configuration requires release review'); }
$uuid=Illuminate\Support\Str::lower(Illuminate\Support\Str::random(24));
queue_application_deployment(application:$a,deployment_uuid:$uuid,commit:'__COMMIT__',force_rebuild:false,is_api:true);
echo json_encode(['deployment_uuid'=>$uuid,'commit'=>'__COMMIT__']);"""
result=subprocess.run(['docker','exec','coolify','php','artisan','tinker','--execute='+php],capture_output=True,text=True)
if result.returncode:raise RuntimeError('Coolify did not accept the release')
record=json.loads(result.stdout)
print(json.dumps(record))
pathlib.Path('/root/.weevcrafts-integrations/current-reviewed-release.json').write_text(json.dumps(record))
p.unlink()
'@
$remoteScript.Replace('__COMMIT__',$releaseCommit).Replace('__DIGEST__',$digest) | ssh -o BatchMode=yes -o ConnectTimeout=8 $Server python3 -
if ($LASTEXITCODE -ne 0) { throw 'Release image activation was not accepted' }
Write-Output 'Image loaded and deployment queued. Verify Coolify status, revision, migrations and readiness before declaring completion.'
