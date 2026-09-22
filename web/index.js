const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const kindCN={worktree:'工作区',commit:'commit',range:'区间',pr:'PR',patch:'补丁'};
let showArch=false; try{ showArch=localStorage.getItem('hudiff.home.arch')==='1'; }catch(e){}
document.getElementById('showarch').checked=showArch;
document.getElementById('showarch').onchange=e=>{ showArch=e.target.checked; try{ localStorage.setItem('hudiff.home.arch', showArch?'1':'0'); }catch(x){} load(); };
function kindMeta(s){
  const m=s.meta||{}; const parts=[`<span>${esc(s.repo.split('/').pop())}</span>`];
  if(s.kind==='pr'){ if(m.url) parts.push(`<a href="${esc(m.url)}" target="_blank" onclick="event.stopPropagation()">${esc(m.url.replace(/^https?:\/\/(www\.)?/,''))}</a>`); if(m.base||m.head) parts.push(`<span><code>${esc(m.head||'?')}</code> → <code>${esc(m.base||'?')}</code></span>`); if(m.author) parts.push(`<span>@${esc(m.author)}</span>`); }
  else if(s.kind==='commit'){ parts.push(`<span><code>${esc(m.rev||'')}</code>${m.base?` ← <code>${esc(m.base)}</code>`:''}</span>`); if(m.author) parts.push(`<span>${esc(m.author)}</span>`); if(m.date) parts.push(`<span>${esc(m.date)}</span>`); if(m.body) parts.push(`<span class="msg" title="${esc(m.body)}">${esc(m.body.split('\n')[0])}</span>`); }
  else if(s.kind==='range'){ parts.push(`<span><code>${esc(m.base||'')}</code> … <code>${esc(m.head||'')}</code></span>`); if(m.branch) parts.push(`<span>分支 <code>${esc(m.branch)}</code></span>`); }
  else if(s.kind==='worktree'){ if(m.branch) parts.push(`<span>分支 <code>${esc(m.branch)}</code></span>`); if(m.base) parts.push(`<span>基线 <code>${esc(m.base)}</code></span>`); parts.push(`<span>${({worktree:'未提交',staged:'已暂存',unstaged:'未暂存'})[m.scope]||''}</span>`); }
  else if(s.kind==='patch'){ if(m.source) parts.push(`<span><code>${esc(m.source)}</code></span>`); }
  parts.push(`<span>${esc(s.created.slice(0,16).replace('T',' '))}</span>`);
  if(s.archived) parts.push(`<span class="chip">已归档</span>`);
  return parts.join('');
}
async function load(){
  const d=await (await fetch('/api/sessions')).json(); const all=d.sessions; const arch=all.filter(s=>s.archived).length; const ss=showArch?all:all.filter(s=>!s.archived);
  document.getElementById('cnt').textContent=`${ss.length} 个会话${arch&&!showArch?` · ${arch} 个已归档`:''} · 端口 ${d.port}`;
  if(!ss.length){ document.getElementById('list').innerHTML='<div class="empty">还没有会话。<br><br><pre>hudiff open            # 工作区改动\nhudiff open HEAD~1     # 某个 commit\nhudiff open pr 123     # GitHub PR</pre></div>'; return; }
  document.getElementById('list').innerHTML=ss.map(s=>{
    const c=s.counts||{}; const chips=[s.waiting?'<span class="chip wait">● agent 在等</span>':'', c.sent?`<span class="chip sent">${c.sent} 条待 agent 处理</span>`:'', c.open?`<span class="chip open">${c.open} 条未发送</span>`:'', c.replied?`<span class="chip">${c.replied} 条已回复</span>`:'', !s.noted?'<span class="chip">还没有说明</span>':''].join('');
    return `<div class="card ${s.archived?'archived':''}" data-href="/s/${s.id}/">
      <span class="kind ${esc(s.kind)}">${esc(kindCN[s.kind]||s.kind)}</span>
      <span class="title">${esc(s.title)}${chips}</span>
      <span class="right"><span class="pm">${s.files} 个文件 <b class="a">+${s.add}</b> <b class="d">−${s.del_}</b> · ${s.comments} 条评论</span><span><button data-arch="${s.id}" data-to="${s.archived?0:1}" title="${s.archived?'取消归档':'归档: 首页不再列出, 数据保留'}">${s.archived?'取消归档':'归档'}</button> <button data-id="${s.id}" title="删除会话(含评论)">删除</button></span></span>
      <span class="meta">${kindMeta(s)}</span></div>`;
  }).join('');
  document.querySelectorAll('.card[data-href]').forEach(c=>c.onclick=e=>{ if(e.target.closest('a,button')) return; location.href=c.dataset.href; });
  document.querySelectorAll('button[data-id]').forEach(b=>b.onclick=async e=>{ e.preventDefault(); e.stopPropagation(); if(!(await dlg.confirm('删除这个会话?', '说明、图解、评论、审查报告一起删, 不可恢复。只是不想在首页看到的话, 用「归档」。', {ok:'删除', danger:true}))) return; await fetch('/api/s/'+b.dataset.id,{method:'DELETE'}); load(); });
  document.querySelectorAll('button[data-arch]').forEach(b=>b.onclick=async e=>{ e.preventDefault(); e.stopPropagation(); await fetch('/api/s/'+b.dataset.arch,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({archived:b.dataset.to==='1'})}); load(); });
}
load(); setInterval(load, 3000);
