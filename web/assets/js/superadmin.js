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
