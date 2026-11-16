#!/usr/bin/env python3
"""Public-key encrypted recovery jobs. Never prints provider bodies or secrets."""
import argparse
import datetime
import fcntl
import hashlib
import json
import os
import pathlib
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import time

ID = re.compile(r"weevcrafts-ops-\d{8}T\d{6}Z(?:-\d+)?\Z")

def valid_id(value):
    return isinstance(value, str) and bool(ID.fullmatch(value))

def atomic_json(path, value):
    partial = pathlib.Path(str(path)+'.partial')
    with partial.open('w') as f:
        json.dump(value, f, indent=2)
        f.write('\n')
        f.flush()
        os.fsync(f.fileno())
    os.chmod(partial, 0o600)
    os.replace(partial, path)

def sha(path):
    h=hashlib.sha256()
    with path.open('rb') as f:
        for b in iter(lambda:f.read(1024*1024),b''):h.update(b)
    return h.hexdigest()

def records(root):
    result=[]
    for p in root.glob('weevcrafts-ops-*.json'):
        try:
            value=json.loads(p.read_text())
            if value.get('format')==1 and valid_id(value.get('id')) and p.stem==value['id'] and (root/(p.stem+'.tar.age')).is_file():
                result.append(value)
        except (OSError, ValueError, KeyError):
            continue
    return sorted(result,key=lambda x:x['id'])

def prune(root, keep=30):
    # Retention is daily, not a count of successful retries/rehearsals that day.
    values=records(root)
    dates=sorted({r['id'][15:23] for r in values})
    retained={max(r['id'] for r in values if r['id'][15:23]==day) for day in dates[-keep:]}
    for r in values:
        if r['id'] not in retained:
            (root/(r['id']+'.tar.age')).unlink(missing_ok=True)
            (root/(r['id']+'.json')).unlink(missing_ok=True)

def staleness(value, now=None):
    now=time.time() if now is None else now
    errors=[]
    if now-value.get('completed_epoch',0)>26*3600:errors.append('server_backup_stale')
    ack=value.get('local_copy_ack')
    if not ack:errors.append('local_copy_missing')
    elif now-ack.get('source_completed_epoch',ack.get('at_epoch',0))>26*3600:errors.append('local_copy_stale')
    return errors

def encrypt_pipeline(producer, encryptor, target, env):
    partial=pathlib.Path(str(target)+'.partial')
    p=q=None
    try:
        with partial.open('wb') as out:
            os.chmod(partial,0o600)
            p=subprocess.Popen(producer,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,env=env)
            q=subprocess.Popen(encryptor,stdin=p.stdout,stdout=out,stderr=subprocess.DEVNULL,env=env)
            p.stdout.close()
            b=q.wait(timeout=1500)
            a=p.wait(timeout=60)
            out.flush();os.fsync(out.fileno())
        if a or b or partial.stat().st_size<64:
            raise RuntimeError('producer/encryption failed; partial archive rejected')
        os.replace(partial,target)
    except BaseException:
        for process in [p,q]:
            if process and process.poll() is None:process.kill();process.wait()
        partial.unlink(missing_ok=True)
        raise

def checked(args, **kw):
    p=subprocess.run(args,capture_output=True,check=False,**kw)
    if p.returncode:raise RuntimeError('required recovery subprocess failed')
    return p.stdout

def configuration(config):
    required=['directory','age_recipient_file','helper_binary','postgres_image','docker_network','application_uuid']
    if any(not config.get(k) for k in required):raise RuntimeError('backup configuration incomplete')
    root=pathlib.Path(config['directory'])
    if not root.is_absolute() or root.name!='scheduled':raise RuntimeError('use a dedicated absolute scheduled directory')
    root.mkdir(parents=True,exist_ok=True,mode=0o700);os.chmod(root,0o700)
    return root

def capture_private_config(app, directory, config):
    (directory/'application-inspect.json').write_text(json.dumps(app))
    (directory/'release.json').write_text(json.dumps({'image_id':app['Image'],'image_ref':app['Config']['Image']}))
    for i,name in enumerate(config.get('configuration_paths',[])):
        source=pathlib.Path(name)
        if not source.is_absolute() or not source.exists():raise RuntimeError('configured recovery source is unavailable')
        if source.is_symlink():raise RuntimeError('recovery sources cannot be symlinks')
        destination=directory/f'source-{i}'
        if source.is_dir():shutil.copytree(source,destination,symlinks=True)
        else:shutil.copyfile(source,destination)
    # Use the model's decrypted environment values only inside encrypted export.
    php=r'''$a=App\Models\Application::where('uuid',getenv('BACKUP_APP_UUID'))->firstOrFail(); echo json_encode(['application'=>$a->toArray(),'settings'=>$a->settings->toArray(),'environment'=>$a->environment_variables->map(function($v){$x=$v->toArray();$x['value']=$v->value;$x['real_value']=$v->real_value;return $x;})]);'''
    env=dict(os.environ,BACKUP_APP_UUID=config['application_uuid'])
    raw=checked(['docker','exec','-e','BACKUP_APP_UUID','coolify','php','artisan','tinker','--execute='+php],env=env)
    json.loads(raw)
    (directory/'coolify-export.json').write_bytes(raw)
    for p in directory.rglob('*'):
        if p.is_file():os.chmod(p,0o600)

def backup(config):
    root=configuration(config)
    recipient=pathlib.Path(config['age_recipient_file']).read_text().strip()
    if not recipient.startswith('age1') or '\n' in recipient:raise RuntimeError('use an age public recipient, never a private identity')
    ids=checked(['docker','ps','-q','--filter','label=coolify.applicationUuid='+config['application_uuid']],text=True).split()
    if len(ids)!=1:raise RuntimeError('expected one running application')
    app=json.loads(checked(['docker','inspect',ids[0]],text=True))[0]
    env=dict(os.environ)
    env.update(dict(x.split('=',1) for x in app['Config'].get('Env',[]) if '=' in x))
    if env.get('WECRATFS_ENV')!='production':raise RuntimeError('scheduled source must be identified production application')
    env['SOURCE_COMMIT']=app['Config']['Image'].split(':')[-1]
    helper=pathlib.Path(config['helper_binary']).resolve()
    if not helper.is_file():raise RuntimeError('recovery helper unavailable')
    private_root=pathlib.Path(config.get('private_work_directory','/run/weevcrafts-backup'))
    private_root.mkdir(parents=True,exist_ok=True,mode=0o700);os.chmod(private_root,0o700)
    with tempfile.TemporaryDirectory(prefix='capture-',dir=private_root) as tmp:
        directory=pathlib.Path(tmp)
        capture_private_config(app,directory,config)
        env['BACKUP_CONFIG_DIRECTORY']='/private-config'
        names=['DATABASE_URL','DATABASE_CA_CERT','S3_ADDR','S3_ACCESS_KEY','S3_SECRET_KEY','S3_BUCKET','S3_SECURE','S3_CA_CERT','S3_REGION','SOURCE_COMMIT','BACKUP_CONFIG_DIRECTORY']
        command=['docker','run','--rm','--name','weevcrafts-scheduled-backup','--label','com.weevcrafts.task=recovery','--network',config['docker_network'],'--cpus','0.3','--memory','512m','--read-only','--tmpfs','/tmp:rw,size=256m,mode=1777','-v',str(helper)+':/weev-backup:ro','-v',str(directory)+':/private-config:ro']
        for name in names:command.extend(['-e',name])
        command.extend(['--entrypoint','/weev-backup',config['postgres_image'],'collect'])
        last=None
        for attempt in range(3):
            ident='weevcrafts-ops-'+datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')+f'-{attempt}'
            target=root/(ident+'.tar.age')
            try:
                encrypt_pipeline(command,['age','-r',recipient],target,env)
                value={'format':1,'id':ident,'completed_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'completed_epoch':time.time(),'sha256':sha(target),'bytes':target.stat().st_size,'image_id':app['Image'],'source_commit':env['SOURCE_COMMIT'],'verification':'producer snapshot/reference/object checks and age encryption completed; independent decrypt/restore separate'}
                atomic_json(root/(ident+'.json'),value)
                prune(root,30)
                print(json.dumps({'result':'backup_complete','id':ident,'bytes':value['bytes']}))
                return
            except Exception as e:
                last=e
                checked(['docker','rm','-f','weevcrafts-scheduled-backup']) if subprocess.run(['docker','inspect','weevcrafts-scheduled-backup'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0 else None
                if attempt<2:time.sleep([60,120][attempt])
        raise RuntimeError('three backup attempts failed; previous verified archives preserved') from last

def acknowledge(config,ident,digest):
    if not valid_id(ident) or not re.fullmatch('[a-f0-9]{64}',digest):raise RuntimeError('invalid acknowledgement')
    root=configuration(config);p=root/(ident+'.json');r=json.loads(p.read_text())
    if r['id']!=ident or r['sha256']!=digest or sha(root/(ident+'.tar.age'))!=digest:raise RuntimeError('acknowledgement checksum mismatch')
    r['local_copy_ack']={'at_epoch':time.time(),'at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sha256':digest,'verified':'local ciphertext hash and age decrypted payload manifest'}
    atomic_json(p,r)

def remote(config,script):
    return checked(['ssh','-o','BatchMode=yes','-o','ConnectTimeout=15',config['remote'],'python3','-'],input=script,text=True)

def pull(config):
    root=pathlib.Path(config['local_directory'])
    if not root.is_absolute() or root.name!='scheduled':raise RuntimeError('use a dedicated absolute local scheduled directory')
    root.mkdir(parents=True,exist_ok=True,mode=0o700);os.chmod(root,0o700)
    folder=config['remote_directory']
    if folder!='/root/weevcrafts-backups/scheduled':raise RuntimeError('unexpected remote backup scope')
    script='import pathlib,json\np=pathlib.Path('+repr(folder)+')\nprint(json.dumps([json.loads(f.read_text()) for f in p.glob("weevcrafts-ops-*.json")]))\n'
    values=json.loads(remote(config,script))
    for r in sorted(values,key=lambda v:v.get('id','')):
        ident=r.get('id','')
        if not valid_id(ident):raise RuntimeError('remote archive identifier invalid')
        target=root/(ident+'.tar.age')
        if not target.exists():
            partial=pathlib.Path(str(target)+'.partial')
            checked(['scp','-q',config['remote']+':'+folder+'/'+ident+'.tar.age',str(partial)])
            if partial.stat().st_size!=r['bytes'] or sha(partial)!=r['sha256']:partial.unlink(missing_ok=True);raise RuntimeError('download checksum mismatch')
            os.chmod(partial,0o600);os.replace(partial,target)
        if sha(target)!=r['sha256']:raise RuntimeError('existing local backup corrupted')
        # Verify authenticated decryption and every payload member before ACK.
        verified=root/(ident+'.json')
        if not verified.exists():
            with tempfile.TemporaryDirectory(prefix='weev-restore-check-',dir=config.get('private_work_directory','/dev/shm')) as tmp:
                first=subprocess.Popen([config.get('age_binary','age'),'-d','-i',config['identity_file'],str(target)],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
                second=subprocess.Popen([config['helper_binary'],'extract-verify',tmp],stdin=first.stdout,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
                first.stdout.close();b=second.wait(timeout=1500);a=first.wait(timeout=60)
                if a or b:raise RuntimeError('local authenticated payload verification failed')
            atomic_json(verified,r)
        args=[config['remote_runner'],'--config',config['remote_config'],'acknowledge',ident,r['sha256']]
        checked(['ssh','-o','BatchMode=yes',config['remote'],shlex.join(args)])
    prune(root,30)
    print(json.dumps({'result':'local_copies_verified','archives':len(values)}))

def status(config):
    values=records(configuration(config));value=values[-1] if values else {}
    acknowledged=[r for r in values if r.get('local_copy_ack')]
    if acknowledged:
        newest=max(acknowledged,key=lambda r:r.get('completed_epoch',0))
        value=dict(value,local_copy_ack=dict(newest['local_copy_ack'],source_completed_epoch=newest['completed_epoch']))
    issues=staleness(value)
    print(json.dumps({'result':'healthy' if not issues else 'attention','issues':issues,'latest_id':value.get('id')}))
    return 0 if not issues else 1

def main():
    os.umask(0o077)
    parser=argparse.ArgumentParser()
    parser.add_argument('--config',required=True)
    parser.add_argument('operation',choices=['backup','pull','status','acknowledge'])
    parser.add_argument('values',nargs='*')
    a=parser.parse_args();config=json.loads(pathlib.Path(a.config).read_text())
    lock=pathlib.Path(a.config+'.lock')
    with lock.open('a') as f:
        fcntl.flock(f,fcntl.LOCK_EX)
        if a.operation=='backup':
            try:
                backup(config)
            except Exception:
                notify=config.get('notify_command')
                if notify:subprocess.run([notify,'backup_failed'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=45)
                raise
        elif a.operation=='pull':pull(config)
        elif a.operation=='status':
            result=status(config)
            if result and config.get('notify_command'):subprocess.run([config['notify_command'],'backup_stale'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=45)
            return result
        else:
            if len(a.values)!=2:raise RuntimeError('acknowledgement requires identifier and hash')
            acknowledge(config,*a.values)
    return 0

if __name__=='__main__':
    try:sys.exit(main())
    except Exception:
        print('Recovery job failed; inspect restricted operational evidence, not provider bodies or credentials.',file=sys.stderr)
        sys.exit(1)
