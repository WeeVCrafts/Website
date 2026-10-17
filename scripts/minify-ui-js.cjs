const fs=require('node:fs/promises');const path=require('node:path');const esbuild=require('esbuild');
async function main(){
 const root=path.resolve(__dirname,'../web/assets/js');
 for(const name of ['ui-standards','experience','quick-view','backend','firebase-client','storefront','product-media','seller-workflows','storefront-editor','storefront-preview']){
  const source=await fs.readFile(path.join(root,name+'.js'),'utf8');
  const result=await esbuild.transform(source,{minify:true,target:'es2020',legalComments:'inline'});
  await fs.writeFile(path.join(root,name+'.min.js'),result.code);
  console.log(name+': '+Buffer.byteLength(source)+' -> '+Buffer.byteLength(result.code)+' bytes');
 }
}
main().catch(error=>{console.error(error);process.exitCode=1;});
