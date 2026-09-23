const $=id=>document.getElementById(id);
const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const KIND={worktree:'workspace',pr:'PR',commit:'commit',range:'range',patch:'patch'};
const ATT={waiting:['<i class="dot"></i>agent waiting',s=>s.waiting], unsent:['unsent drafts',s=>s.counts?.open>0], pending:['pending',s=>s.counts?.sent>0]};
const F={repo:'',kind:'',att:''};   // one pick per rail section, '' = no narrowing
let D=null, raw='', sel='', selIdx=0;
const LS={get:(k,d)=>{ try{ const v=localStorage.getItem('hudiff.home.'+k); return v===null?d:JSON.parse(v); }catch(e){ return d; } }, set:(k,v)=>{ try{ localStorage.setItem('hudiff.home.'+k, JSON.stringify(v)); }catch(e){} }};
let showArch=!!LS.get('arch',0), railW=LS.get('railw',208), railOff=LS.get('railoff',null);   // null = never touched: folded under 1280

// a checkout under <repo>.worktrees/ belongs to <repo>'s group
const repoOf=s=>s.repo.replace(/\.worktrees\/.*$/,'');
const tilde=p=>p.replace(/^\/(Users|home)\/[^/]+/,'~');
const h7=s=>String(s||'').slice(0,7);
const dim=(s,c='')=>s?`<span class="dim${c}">${s}</span>`:'';
function age(t){ const m=(Date.now()-Date.parse(t))/6e4; return m<60?Math.max(1,m|0)+'m':m<1440?(m/60|0)+'h':(m/1440|0)+'d'; }
function ref(s){
  const m=s.meta||{};
  if(s.kind==='worktree') return `<span class="b">${esc(m.branch)}</span>`+dim(m.base&&'base '+esc(h7(m.base)))+dim({worktree:'uncommitted',staged:'staged',unstaged:'unstaged'}[m.scope]);
  if(s.kind==='commit'){ const day=m.date&&new Date(m.date+'T00:00').toLocaleDateString('en-US',{month:'short',day:'numeric'});
    return `<span>${esc(h7(m.rev))}${m.base?` ${dim('←')} ${esc(h7(m.base))}`:''}</span>`+dim(esc([m.author,day].filter(Boolean).join(' · ')),' b'); }
  if(s.kind==='pr'){ const n=/\/pull\/(\d+)/.exec(m.url||'');
    return (n?`<a href="${esc(m.url)}" target="_blank">#${n[1]}</a>`:'')+`<span class="b">${esc(m.head)} ${dim('→')} ${esc(m.base)}</span>`+dim(m.author&&'@'+esc(m.author)); }
  if(s.kind==='range') return `<span class="b">${esc(m.base)} ${dim('…')} ${esc(m.head)}</span>`+dim(esc(m.branch));
  return `<span class="b">${esc(m.source)}</span>`;
}
function chips(s){
  const c=s.counts||{};
  return (s.waiting?'<span class="chip wait"><i class="dot"></i>agent waiting</span>':'')+(c.open?`<span class="chip draft">${c.open} unsent</span>`:'')
    +(c.sent?`<span class="chip pend">${c.sent} pending</span>`:'')+(c.replied?`<span class="chip">${c.replied} replied</span>`:'')+(s.noted?'':'<span class="chip none">no notes yet</span>');
}
function row(s){
  const t=s.add+s.del_||1;
  return `<div class="lr${s.archived?' archived':''}" data-id="${esc(s.id)}"><span class="pill ${esc(s.kind)}">${esc(KIND[s.kind]||s.kind)}</span>
    <div class="t" title="${esc(s.title)}">${esc(s.title)}</div><div class="ref mono">${ref(s)}</div><div class="st">${chips(s)}</div>
    <div class="delta"><span class="bar"><i class="g" style="width:${s.add/t*100}%"></i><i class="r" style="width:${s.del_/t*100}%"></i></span><span class="mono n"><b class="a">+${s.add}</b> <b class="d">−${s.del_}</b></span><span class="dim mono f">${s.files}f</span></div>
    <span class="age dim" title="${esc(s.created.slice(0,16).replace('T',' '))}">${age(s.created)}</span>
    <div class="more"><button class="ic" title="More actions">···</button><div class="menu"><button data-act="arch">${s.archived?'Unarchive':'Archive'}</button><button data-act="del">Delete</button></div></div></div>`;
}
// skip = the rail section being counted, so its own pick does not zero its siblings
function hit(s,skip){
  if(s.archived&&!showArch) return false;
  if(skip!=='repo'&&F.repo&&repoOf(s)!==F.repo) return false;
  if(skip!=='kind'&&F.kind&&s.kind!==F.kind) return false;
  if(skip!=='att'&&F.att&&!ATT[F.att][1](s)) return false;
  const q=$('q').value.trim().toLowerCase(), m=s.meta||{};
  return !q||[s.title,s.repo,m.branch,m.head,m.base,m.rev,m.author,m.url,m.source].some(x=>String(x??'').toLowerCase().includes(q));
}
function render(){
  const all=D.sessions, arch=all.filter(s=>s.archived).length, pool=all.filter(s=>showArch||!s.archived);
  if(!all.length){ $('cnt').textContent=`0 sessions · port ${D.port}`; $('rail').innerHTML=''; $('list').innerHTML='<div class="empty">No sessions yet.<br><br><pre>hudiff                 # working tree\nhudiff commit HEAD~1   # one commit\nhudiff pr 123          # a GitHub PR</pre></div>'; return; }
  const item=(k,v,label,f,title='')=>`<a href="#" class="${F[k]===v?'on':''}" data-k="${k}" data-v="${esc(v)}" title="${esc(title)}"><span class="l">${label}</span><span class="n">${pool.filter(s=>hit(s,k)&&f(s)).length}</span></a>`;
  const repos=[...new Set(pool.map(repoOf))];
  $('rail').innerHTML='<div class="lbl">Repos</div>'+item('repo','','All',()=>true)+repos.map(r=>item('repo',r,esc(r.split('/').pop()),s=>repoOf(s)===r,tilde(r))).join('')
    +'<div class="lbl">Kind</div>'+Object.keys(KIND).filter(k=>pool.some(s=>s.kind===k)).map(k=>item('kind',k,`<span class="pill ${k}">${KIND[k]}</span>`,s=>s.kind===k)).join('')
    +'<div class="lbl">Attention</div>'+Object.entries(ATT).map(([k,[label,f]])=>item('att',k,label,f)).join('');
  const groups=new Map(), shown=pool.filter(s=>hit(s));
  for(const s of shown){ const r=repoOf(s); if(!groups.has(r)) groups.set(r,[]); groups.get(r).push(s); }
  $('cnt').textContent=`${all.length-arch} sessions${arch?` · ${arch} archived`:''}${shown.length<pool.length?` · ${shown.length} shown`:''} · port ${D.port}`;
  $('list').innerHTML=groups.size?[...groups].map(([r,ss])=>`<div class="gh"><span class="name">${esc(r.split('/').pop())}</span><span class="path mono">${esc(tilde(ss.some(s=>s.repo===r)?r+'/':r+'.worktrees/'))}</span><span class="dim">${ss.length}</span></div><div class="ledger">${ss.map(row).join('')}</div>`).join('')
    :'<div class="empty">Nothing matches the filter.</div>';
  const rows=[...document.querySelectorAll('.lr')], r=rows.find(x=>x.dataset.id===sel)||rows[Math.min(selIdx,rows.length-1)];
  if(r) pick(r,false); else sel='';
}
function pick(r,scroll=true){
  document.querySelector('.lr.sel')?.classList.remove('sel'); r.classList.add('sel');
  sel=r.dataset.id; selIdx=[...document.querySelectorAll('.lr')].indexOf(r);
  if(scroll) r.scrollIntoView({block:'nearest'});
}
function move(d){ const rows=[...document.querySelectorAll('.lr')], i=rows.findIndex(x=>x.dataset.id===sel); const r=rows[Math.max(0,Math.min(rows.length-1,i+d))]; if(r) pick(r); }
const go=(id,tab)=>tab?window.open('/s/'+id+'/'):location.href='/s/'+id+'/';
const closeMenus=()=>document.querySelectorAll('.more.open').forEach(m=>m.classList.remove('open'));
const api=(id,method,body)=>fetch('/api/s/'+id,{method,headers:{'Content-Type':'application/json'},body:body&&JSON.stringify(body)});
async function archive(id){ const s=D.sessions.find(x=>x.id===id); if(!s) return; await api(id,'PATCH',{archived:!s.archived}); load(); }
async function del(id){
  if(!(await dlg.confirm('Delete this session?', 'The notes, diagrams, comments and review report go with it. This cannot be undone. To just hide it from the home page, archive it instead.', {ok:'Delete', danger:true}))) return;
  await api(id,'DELETE'); load();
}
async function load(){
  if(document.querySelector('.more.open')) return;   // a refresh would pull the menu out from under the pointer
  const txt=await (await fetch('/api/sessions')).text(), stamp=txt+(Date.now()/6e4|0);   // at least once a minute, so the ages move
  if(stamp===raw) return; raw=stamp;
  D=JSON.parse(txt); D.sessions.sort((a,b)=>b.created.localeCompare(a.created)); render();
}

$('showarch').checked=showArch;
$('showarch').onchange=e=>{ showArch=e.target.checked; LS.set('arch', showArch?1:0); render(); };
$('q').oninput=()=>{ sel=''; selIdx=0; render(); };
function applyRail(){ const off=railOff??innerWidth<1280; $('wrap').style.setProperty('--rail-w', railW+'px'); document.body.classList.toggle('rail-off', off); $('tog').textContent=off?'›':'‹'; }
const toggleRail=()=>{ railOff=!document.body.classList.contains('rail-off'); LS.set('railoff', railOff); applyRail(); };
$('tog').onclick=e=>{ e.stopPropagation(); toggleRail(); };
$('sash').ondblclick=toggleRail;
$('sash').onmousedown=e=>{ if(e.target.tagName==='BUTTON') return; e.preventDefault(); const el=$('sash'), left=$('wrap').getBoundingClientRect().left;
  el.classList.add('drag'); document.body.classList.add('dragging');
  const move=ev=>{ const w=ev.clientX-left; if(railOff!==false&&w>140){ railOff=false; LS.set('railoff', false); } railW=Math.round(Math.max(160, Math.min(w, 420))); applyRail(); };
  const up=()=>{ document.removeEventListener('mousemove',move); document.removeEventListener('mouseup',up); el.classList.remove('drag'); document.body.classList.remove('dragging'); LS.set('railw', railW); };
  document.addEventListener('mousemove',move); document.addEventListener('mouseup',up); };
addEventListener('resize', ()=>{ if(railOff===null) applyRail(); });
applyRail();
$('rail').onclick=e=>{ const a=e.target.closest('a[data-k]'); if(!a) return; e.preventDefault();
  F[a.dataset.k]=F[a.dataset.k]===a.dataset.v?'':a.dataset.v; sel=''; selIdx=0; render(); };
$('list').onclick=e=>{
  const r=e.target.closest('.lr'), b=e.target.closest('button'); if(!r) return;
  if(b){ const m=b.closest('.more'), was=m.classList.contains('open'); closeMenus();
    if(b.classList.contains('ic')){ if(!was) m.classList.add('open'); pick(r,false); }
    else (b.dataset.act==='arch'?archive:del)(r.dataset.id);
    return; }
  if(!e.target.closest('a,.menu')) go(r.dataset.id, e.metaKey||e.ctrlKey);
};
document.addEventListener('click',e=>{ if(!e.target.closest('.more')) closeMenus(); });
document.addEventListener('keydown',e=>{
  if($('dlg')?.classList.contains('on')||e.metaKey||e.ctrlKey||e.altKey) return;
  const q=$('q'), inQ=document.activeElement===q;
  if(e.key==='Escape'){ closeMenus(); if(q.value){ q.value=''; q.oninput(); } q.blur(); return; }
  if(e.key==='ArrowDown'||e.key==='ArrowUp'){ e.preventDefault(); move(e.key==='ArrowDown'?1:-1); return; }
  if(e.key==='Enter'){ if((inQ||document.activeElement===document.body)&&sel) go(sel); return; }
  if(inQ) return;
  if(e.key==='/'){ e.preventDefault(); q.focus(); }
  else if(e.key==='j') move(1);
  else if(e.key==='k') move(-1);
  else if(e.key==='a'&&sel) archive(sel);
  else if(e.key==='[') toggleRail();
});
load(); setInterval(load, 3000);
