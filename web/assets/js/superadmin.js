/* Super Admin presentation only; mutations stay with existing form owners. */
(() => {
  'use strict';
  const shell=document.querySelector('.superadmin-body .admin-shell');
  if(!shell) return;
  const toggle=shell.querySelector('[data-superadmin-toggle]');
  const desktop=matchMedia('(min-width: 961px)');
  const key='weevcrafts.superadmin.sidebar-collapsed';
  let collapsed=false;
  try {collapsed=localStorage.getItem(key)==='true';} catch {}
  function render(){
    shell.classList.toggle('is-collapsed',collapsed&&desktop.matches);
    if(toggle){toggle.setAttribute('aria-expanded',String(!collapsed));toggle.setAttribute('aria-label',collapsed?'Expand sidebar':'Collapse sidebar');}
  }
  toggle?.addEventListener('click',()=>{collapsed=!collapsed;try{localStorage.setItem(key,String(collapsed));}catch{}render();});
  desktop.addEventListener('change',()=>{
    render();
    if(desktop.matches){
      const close=shell.querySelector('.admin-sidebar-close');
      if(shell.querySelector('.admin-sidebar.is-open')) close?.click();
    }
  });
  render();
  const categoryForm=shell.querySelector('[data-category-publication]');
  if(categoryForm){
    const name=categoryForm.elements.namedItem('name'),slug=categoryForm.elements.namedItem('id'),file=categoryForm.elements.namedItem('image_file');
    const preview=categoryForm.querySelector('[data-category-preview]'),caption=categoryForm.querySelector('[data-category-preview-name]'),message=categoryForm.querySelector('[data-category-file]');
    const savedImage=preview.getAttribute('src');let objectURL,slugEdited=!!slug.value,sequence=0;
    slug.addEventListener('input',()=>{slugEdited=true;});
    name.addEventListener('input',()=>{caption.textContent=name.value;if(!slug.readOnly&&!slugEdited)slug.value=name.value.normalize('NFKD').replace(/[\u0300-\u036f]/g,'').toLowerCase().replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,'').slice(0,120);});
    function resetPreview(){++sequence;if(objectURL){URL.revokeObjectURL(objectURL);objectURL=null;}preview.src=savedImage||'';preview.hidden=!savedImage;caption.textContent=name.value;message.textContent='';file.setCustomValidity('');}
    file.addEventListener('change',()=>{
      resetPreview();const selected=file.files[0];if(!selected)return;
      const current=sequence;
      if(!['image/jpeg','image/png','image/webp'].includes(selected.type)||!selected.size||selected.size>8*1024*1024){file.setCustomValidity('Choose a JPEG, PNG or WebP photograph up to 8 MiB.');message.textContent=file.validationMessage;return;}
      message.textContent=selected.name+' · '+Math.ceil(selected.size/1024)+' KiB selected';
      objectURL=URL.createObjectURL(selected);const candidate=new Image();
      candidate.onload=()=>{if(current!==sequence)return;if(candidate.naturalWidth>8192||candidate.naturalHeight>8192||candidate.naturalWidth*candidate.naturalHeight>16000000){file.setCustomValidity('Choose a photograph up to 16 megapixels and 8192 pixels per side.');message.textContent=file.validationMessage;return;}preview.src=objectURL;preview.hidden=false;};
      candidate.onerror=()=>{if(current===sequence){file.setCustomValidity('This photograph could not be read. Choose a different image.');message.textContent=file.validationMessage;}};
      candidate.src=objectURL;
    });
    categoryForm.addEventListener('ui:discard',resetPreview);
    window.addEventListener('pagehide',()=>{if(objectURL)URL.revokeObjectURL(objectURL);});
  }
  // Supply the same explicit labels for native records, including shared detail
  // tables; the source remains one semantic table at every viewport.
  function labelTables(root){
    root.querySelectorAll('table.ui-table').forEach(table=>{
      table.classList.add('ui-table--records');
      const labels=Array.from(table.querySelectorAll('thead th'),th=>th.textContent.trim());
      table.querySelectorAll('tbody tr').forEach(row=>Array.from(row.children).forEach((cell,i)=>{if(!cell.hasAttribute('data-label')&&labels[i])cell.dataset.label=labels[i];}));
    });
  }
  labelTables(shell);
  document.body.addEventListener('htmx:afterSwap',()=>labelTables(shell));
})();
