const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const KIND={worktree:'worktree',commit:'commit',range:'range',pr:'PR',patch:'patch'};
// Show who wrote the comments only when more than one author did.
function authorBreakdown(a){
  const names=Object.keys(a||{});
  if(names.length < 2) return '';
  return ' (' + names.map(n=>`${esc(n==='user'?'you':n)} ${a[n]}`).join(', ') + ')';
}
let showArch=false; try{ showArch=localStorage.getItem('hudiff.home.arch')==='1'; }catch(e){}
document.getElementById('showarch').checked=showArch;
document.getElementById('showarch').onchange=e=>{ showArch=e.target.checked; try{ localStorage.setItem('hudiff.home.arch', showArch?'1':'0'); }catch(x){} load(); };
function kindMeta(s){
  const m=s.meta||{}; const parts=[`<span>${esc(s.repo.split('/').pop())}</span>`];
  if(s.kind==='pr'){ if(m.url) parts.push(`<a href="${esc(m.url)}" target="_blank" onclick="event.stopPropagation()">${esc(m.url.replace(/^https?:\/\/(www\.)?/,''))}</a>`); if(m.base||m.head) parts.push(`<span><code>${esc(m.head||'?')}</code> → <code>${esc(m.base||'?')}</code></span>`); if(m.author) parts.push(`<span>@${esc(m.author)}</span>`); }
  else if(s.kind==='commit'){ parts.push(`<span><code>${esc(m.rev||'')}</code>${m.base?` ← <code>${esc(m.base)}</code>`:''}</span>`); if(m.author) parts.push(`<span>${esc(m.author)}</span>`); if(m.date) parts.push(`<span>${esc(m.date)}</span>`); if(m.body) parts.push(`<span class="msg" title="${esc(m.body)}">${esc(m.body.split('\n')[0])}</span>`); }
  else if(s.kind==='range'){ parts.push(`<span><code>${esc(m.base||'')}</code> … <code>${esc(m.head||'')}</code></span>`); if(m.branch) parts.push(`<span>branch <code>${esc(m.branch)}</code></span>`); }
  else if(s.kind==='worktree'){ if(m.branch) parts.push(`<span>branch <code>${esc(m.branch)}</code></span>`); if(m.base) parts.push(`<span>base <code>${esc(m.base)}</code></span>`); parts.push(`<span>${({worktree:'uncommitted',staged:'staged',unstaged:'unstaged'})[m.scope]||''}</span>`); }
  else if(s.kind==='patch'){ if(m.source) parts.push(`<span><code>${esc(m.source)}</code></span>`); }
  parts.push(`<span>${esc(s.created.slice(0,16).replace('T',' '))}</span>`);
  if(s.archived) parts.push(`<span class="chip">archived</span>`);
  return parts.join('');
}
async function load(){
  const d=await (await fetch('/api/sessions')).json(); const all=d.sessions; const arch=all.filter(s=>s.archived).length; const ss=showArch?all:all.filter(s=>!s.archived);
  document.getElementById('cnt').textContent=`${ss.length} sessions${arch&&!showArch?` · ${arch} archived`:''} · port ${d.port}`;
  if(!ss.length){ document.getElementById('list').innerHTML='<div class="empty">No sessions yet.<br><br><pre>hudiff                 # working tree\nhudiff commit HEAD~1   # one commit\nhudiff pr 123          # a GitHub PR</pre></div>'; return; }
  document.getElementById('list').innerHTML=ss.map(s=>{
    const c=s.counts||{}; const chips=[s.waiting?'<span class="chip wait">● agent waiting</span>':'', c.sent?`<span class="chip sent">${c.sent} pending</span>`:'', c.open?`<span class="chip open">${c.open} unsent</span>`:'', c.replied?`<span class="chip">${c.replied} replied</span>`:'', !s.noted?'<span class="chip">no notes yet</span>':''].join('');
    return `<div class="card ${s.archived?'archived':''}" data-href="/s/${s.id}/">
      <span class="kind ${esc(s.kind)}">${esc(KIND[s.kind]||s.kind)}</span>
      <span class="title">${esc(s.title)}${chips}</span>
      <span class="right"><span class="pm">${s.files} files <b class="a">+${s.add}</b> <b class="d">−${s.del_}</b> · ${s.comments} comments${authorBreakdown(s.authors)}</span><span><button data-arch="${s.id}" data-to="${s.archived?0:1}" title="${s.archived?'unarchive':'archive: hidden from the home page, data kept'}">${s.archived?'Unarchive':'Archive'}</button> <button data-id="${s.id}" title="delete the session and its comments">Delete</button></span></span>
      <span class="meta">${kindMeta(s)}</span></div>`;
  }).join('');
  document.querySelectorAll('.card[data-href]').forEach(c=>c.onclick=e=>{ if(e.target.closest('a,button')) return; location.href=c.dataset.href; });
  document.querySelectorAll('button[data-id]').forEach(b=>b.onclick=async e=>{ e.preventDefault(); e.stopPropagation(); if(!(await dlg.confirm('Delete this session?', 'The notes, diagrams, comments and review report go with it. This cannot be undone. To just hide it from the home page, archive it instead.', {ok:'Delete', danger:true}))) return; await fetch('/api/s/'+b.dataset.id,{method:'DELETE'}); load(); });
  document.querySelectorAll('button[data-arch]').forEach(b=>b.onclick=async e=>{ e.preventDefault(); e.stopPropagation(); await fetch('/api/s/'+b.dataset.arch,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({archived:b.dataset.to==='1'})}); load(); });
}
load(); setInterval(load, 3000);
