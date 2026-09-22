'use strict';
const S = {data:null, state:null, active:0, activeHunk:null, showResolved:false, openForms:new Set(), theme:'auto', sumOpen:false, pin:null, tab:'notes', mid:'code', activeDg:null};
const C = { q:'', st:new Set(['open','sent','replied']), au:null };   // comment list: filter text / which statuses to show
const LS = { get(k,d){ try{ const v=localStorage.getItem('hudiff.'+k); return v===null?d:JSON.parse(v); }catch(e){ return d; } }, set(k,v){ try{ localStorage.setItem('hudiff.'+k, JSON.stringify(v)); }catch(e){} } };

// ── Nerd Font icons. Write every code point escaped, never paste the glyph ──
// File type: [glyph, colour], following nvim-web-devicons.
// Use U(0x....): a pasted private-use glyph gets silently dropped somewhere in the editing chain (it has happened).
const U = c => String.fromCharCode(c);
const FT = {
  go:[U(0xe627),'#00add8'], lua:[U(0xe620),'#51a0cf'], json:[U(0xe60b),'#cbcb41'], md:[U(0xe73e),'#519aba'], markdown:[U(0xe73e),'#519aba'], yaml:[U(0xe6a8),'#6d8086'],
  yml:[U(0xe6a8),'#6d8086'], toml:[U(0xe6b2),'#9c4221'], ini:[U(0xe615),'#6d8086'], conf:[U(0xe615),'#6d8086'], cfg:[U(0xe615),'#6d8086'], sh:[U(0xe795),'#89e051'],
  bash:[U(0xe795),'#89e051'], zsh:[U(0xe795),'#89e051'], fish:[U(0xe795),'#4d5a5e'], py:[U(0xe606),'#ffbc03'], js:[U(0xe74e),'#cbcb41'], mjs:[U(0xe74e),'#cbcb41'],
  ts:[U(0xe628),'#519aba'], tsx:[U(0xe7ba),'#519aba'], jsx:[U(0xe7ba),'#20c2e3'], html:[U(0xe736),'#e44d26'], css:[U(0xe749),'#42a5f5'], scss:[U(0xe749),'#f55385'],
  vue:[U(0xe6a0),'#8dc149'], svelte:[U(0xe697),'#ff3e00'], rs:[U(0xe7a8),'#dea584'], java:[U(0xe738),'#cc3e44'], kt:[U(0xe634),'#7f52ff'], c:[U(0xe61e),'#599eff'],
  h:[U(0xe61e),'#a074c4'], cpp:[U(0xe61d),'#519aba'], hpp:[U(0xe61d),'#a074c4'], rb:[U(0xe739),'#701516'], php:[U(0xe73d),'#a074c4'], swift:[U(0xe755),'#e37933'],
  dart:[U(0xe798),'#03589c'], ex:[U(0xe62d),'#a074c4'], erl:[U(0xe7b1),'#b83998'], sql:[U(0xe706),'#dad8d8'], proto:[U(0xe728),'#519aba'], graphql:[U(0xe662),'#e535ab'],
  lock:[U(0xf023),'#bbbbbb'], http:[U(0xf0ac),'#519aba'], rest:[U(0xf0ac),'#519aba'], env:[U(0xf462),'#faf743'], vim:[U(0xe62b),'#019833'], txt:[U(0xf15c),'#89e051'],
  log:[U(0xf18d),'#dddddd'], csv:[U(0xf1c3),'#89e051'], pdf:[U(0xf1c1),'#b30b00'], png:[U(0xf1c5),'#a074c4'], jpg:[U(0xf1c5),'#a074c4'], jpeg:[U(0xf1c5),'#a074c4'],
  gif:[U(0xf1c5),'#a074c4'], svg:[U(0xf1c5),'#ffb13b'], webp:[U(0xf1c5),'#a074c4'], zip:[U(0xf410),'#eca517'], gz:[U(0xf410),'#eca517'], tar:[U(0xf410),'#eca517'],
  dockerfile:[U(0xf308),'#458ee6'], makefile:[U(0xe779),'#6d8086'], gitignore:[U(0xe702),'#f54d27'], gitattributes:[U(0xe702),'#f54d27'], editorconfig:[U(0xe652),'#fff2f2'], plist:[U(0xe615),'#6d8086'],
  xml:[U(0xe619),'#e37933'], mod:[U(0xe627),'#00add8'], sum:[U(0xe627),'#00add8'], tmpl:[U(0xe60e),'#6d8086'], tpl:[U(0xe60e),'#6d8086'],
};
const FT_DEFAULT = [U(0xf15b),'#6d8086'];
const FOLDER = U(0xf07b);
// git status: modified / added / deleted / renamed (FontAwesome boxes, same set as yazi)
const GS = { M:[U(0xf044),'modified'], A:[U(0xf196),'added'], D:[U(0xf147),'Delete'], R:[U(0xf074),'renamed'] };
function iconFor(path){
  const base=path.split('/').pop(); const lower=base.toLowerCase();
  if(/^dockerfile/.test(lower)) return FT.dockerfile; if(/^makefile$/.test(lower)) return FT.makefile;
  if(/^\.tmux\.conf|^kitty\.conf$/.test(lower)) return FT.conf;
  if(/^\.?(bash|zsh)rc$|^\.?(bash|zsh)_profile$|^\.zshenv$/.test(lower)) return FT.sh;
  if(lower==='claude.md'||lower==='agents.md') return FT.md;
  const ext = lower.includes('.') ? lower.split('.').pop() : lower;
  if(FT[ext]) return FT[ext];
  if(!lower.includes('.')){ return ['','#89e051']; }   // no extension usually means a script
  return FT_DEFAULT;
}
const icHTML = (path) => { const [ch,color]=iconFor(path); return `<span class="ic nf" style="color:${color}">${ch}</span>`; };
const gsHTML = (st) => `<span class="gs nf ${st}" title="${GS[st]?.[1]||st}">${GS[st]?.[0]||st}</span>`;
const BASE = location.pathname.replace(/\/+$/,'');   // /s/<id>
const $ = (sel, el=document) => el.querySelector(sel);
const esc = s => String(s??'').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const toast = m => { const t=$('#toast'); t.textContent=m; t.classList.add('on'); clearTimeout(t._t); t._t=setTimeout(()=>t.classList.remove('on'),2000); };

// ── Theme: auto / light / dark, remembered in localStorage ──
function applyTheme(t){
  S.theme=t; try{ localStorage.setItem('hudiff.theme', t); }catch(e){}
  if(t==='auto') delete document.documentElement.dataset.theme; else document.documentElement.dataset.theme=t;
  const dark = t==='dark' || (t==='auto' && matchMedia('(prefers-color-scheme: dark)').matches);
  $('#hljs-light').disabled = dark; $('#hljs-dark').disabled = !dark;
  const b=$('#theme'); if(b) b.textContent = {auto:'◐ auto', light:'☀ light', dark:'☾ dark'}[t];
  if(S.data){ renderDiagrams(); if(S.mid==='diagrams') renderNotes(); }
}
try{ S.theme = localStorage.getItem('hudiff.theme') || 'auto'; }catch(e){}
S.tab = LS.get('tab', 'notes');
S.mid = LS.get('mid', 'code'); { const v=new URLSearchParams(location.search).get('view'); if(v==='code'||v==='diagrams'||v==='review') S.mid=v; } { const t=new URLSearchParams(location.search).get('tab'); if(t==='notes'||t==='comments') S.tab=t; }   // ?tab=comments preselects
S.sumOpen = LS.get('sum.'+location.pathname.split('/')[2], false);

// ── Side panes: drag to resize, fold, remembered in localStorage ──
function setupSash(){
  const main=$('#main');
  const W={nav:LS.get('w.nav',250), aside:LS.get('w.aside',400)};
  const OFF={nav:LS.get('off.nav',false), aside:LS.get('off.aside',false)};
  const qs=new URLSearchParams(location.search); if(qs.has('nav')) OFF.nav = qs.get('nav')==='0'; if(qs.has('aside')) OFF.aside = qs.get('aside')==='0';   // ?nav=0&aside=0 starts folded (not persisted)
  const apply=()=>{ main.style.setProperty('--nav-w', W.nav+'px'); main.style.setProperty('--aside-w', W.aside+'px'); main.classList.toggle('nav-off', OFF.nav); main.classList.toggle('aside-off', OFF.aside); $('#tog-l').textContent=OFF.nav?'›':'‹'; $('#tog-r').textContent=OFF.aside?'‹':'›'; };
  const toggle=side=>{ OFF[side]=!OFF[side]; LS.set('off.'+side, OFF[side]); apply(); };
  [['sash-l','nav'],['sash-r','aside']].forEach(([id,side])=>{
    const el=document.getElementById(id);
    el.querySelector('button').onclick=e=>{ e.stopPropagation(); toggle(side); };
    el.ondblclick=()=>toggle(side);
    el.onmousedown=e=>{ if(e.target.tagName==='BUTTON') return; e.preventDefault(); el.classList.add('drag'); document.body.classList.add('dragging');
      const rect=main.getBoundingClientRect();
      const move=ev=>{ let w = side==='nav' ? ev.clientX-rect.left : rect.right-ev.clientX; w=Math.max(120, Math.min(w, rect.width*0.6)); if(OFF[side] && w>140){ OFF[side]=false; LS.set('off.'+side,false); } W[side]=Math.round(w); apply(); };
      const up=()=>{ document.removeEventListener('mousemove',move); document.removeEventListener('mouseup',up); el.classList.remove('drag'); document.body.classList.remove('dragging'); LS.set('w.'+side, W[side]); };
      document.addEventListener('mousemove',move); document.addEventListener('mouseup',up); };
  });
  apply();
}
setupSash();
matchMedia('(prefers-color-scheme: dark)').addEventListener('change', ()=>applyTheme(S.theme));

// ── Highlighting: pick the language by extension, feed hljs line by line; fall back to plain escaping ──
const LANG = {go:'go',lua:'lua',py:'python',js:'javascript',mjs:'javascript',cjs:'javascript',jsx:'javascript',ts:'typescript',tsx:'typescript',json:'json',sh:'bash',bash:'bash',zsh:'bash',md:'markdown',yaml:'yaml',yml:'yaml',toml:'ini',ini:'ini',conf:'ini',rs:'rust',java:'java',kt:'kotlin',c:'c',h:'c',cpp:'cpp',cc:'cpp',hpp:'cpp',html:'xml',xml:'xml',css:'css',scss:'scss',sql:'sql',rb:'ruby',php:'php',swift:'swift',proto:'protobuf',dockerfile:'dockerfile',makefile:'makefile',diff:'diff',http:'http',lock:'json'};
function langFor(path){
  const base=path.split('/').pop(); const ext=base.includes('.')?base.split('.').pop().toLowerCase():base.toLowerCase();
  if(/^\.tmux\.conf/.test(base)||/^kitty\.conf$/.test(base)) return 'ini';
  if(/^(bash|zsh)rc$|\.(bash|zsh)rc$|^config-|^link-/.test(base)) return 'bash';
  return LANG[ext]||null;
}
function hl(code, lang){
  if(!lang||!window.hljs||!hljs.getLanguage(lang)) return esc(code);
  try{ return hljs.highlight(code,{language:lang, ignoreIllegals:true}).value; }catch(e){ return esc(code); }
}

// Tiny markdown: only what an agent note uses, everything escaped before rendering
function md(src){
  if(!src) return '';
  src = String(src).replace(/\\n/g, '\n');   // agents sometimes write a literal \n; be forgiving
  let out=''; const lines=String(src).split('\n'); let i=0;
  while(i<lines.length){
    const l=lines[i];
    if(/^```/.test(l)){ const buf=[]; i++; while(i<lines.length && !/^```/.test(lines[i])) buf.push(lines[i++]); i++; out+=`<pre><code>${esc(buf.join('\n'))}</code></pre>`; continue; }
    if(/^\s*[-*] /.test(l)){ const items=[]; while(i<lines.length && /^\s*[-*] /.test(lines[i])) items.push(lines[i++].replace(/^\s*[-*] /,'')); out+='<ul>'+items.map(x=>`<li>${inline(x)}</li>`).join('')+'</ul>'; continue; }
    if(/^\s*\d+\. /.test(l)){ const items=[]; while(i<lines.length && /^\s*\d+\. /.test(lines[i])) items.push(lines[i++].replace(/^\s*\d+\. /,'')); out+='<ol>'+items.map(x=>`<li>${inline(x)}</li>`).join('')+'</ol>'; continue; }
    if(/^\s*$/.test(l)){ i++; continue; }
    const para=[]; while(i<lines.length && !/^\s*$/.test(lines[i]) && !/^```/.test(lines[i]) && !/^\s*[-*] /.test(lines[i]) && !/^\s*\d+\. /.test(lines[i])) para.push(lines[i++]);
    out+=`<p>${inline(para.join('\n'))}</p>`;
  }
  return out;
}
function inline(s){
  return esc(s).replace(/`([^`]+)`/g,'<code>$1</code>').replace(/\*\*([^*]+)\*\*/g,'<b>$1</b>').replace(/\n/g,'<br>');
}
const tagClass = t => 'tag t'+((Array.from(String(t)).reduce((a,c)=>a+c.codePointAt(0),0))%5);

async function api(method, path, body){
  const r = await fetch(BASE+path, {method, headers:{'Content-Type':'application/json'}, body: body!==undefined?JSON.stringify(body):undefined});
  if(!r.ok){ toast('error: '+(await r.text())); throw new Error(path); }
  return r.json();
}

let treeReady=false;
async function loadAll(){
  S.data = await api('GET','/api/session'); S.state = S.data.state;
  if(!treeReady){ treeReady=true; T.closed=new Set(LS.get('closed.'+S.data.session.id, [])); setupTree(); }
  renderHeader(); renderTree(); renderDiff(); renderComments(); renderMidbar(); renderDiagrams(); renderReview(); markReviewLines(); renderNotes();
  refreshWaiting().catch(()=>{});
  loadKeymap();
}

const kindCN = {worktree:'working tree', commit:'commit', range:'range', pr:'PR', patch:'patch'};
function renderHeader(){
  const {session:s, diff, notes} = S.data;
  const adds = diff.files.reduce((a,f)=>a+f.add,0), dels = diff.files.reduce((a,f)=>a+f.del,0), hunks = diff.files.reduce((a,f)=>a+f.hunks.length,0);
  const m = s.meta||{};
  const parts = [`<span>${diff.files.length} files · ${hunks} hunks</span>`, `<span><b class="add">+${adds}</b> <b class="del">−${dels}</b></span>`];
  if(m.rev) parts.push(`<span><code>${esc(m.rev)}</code>${m.base?` ← <code>${esc(m.base)}</code>`:''}</span>`);
  else if(m.base) parts.push(`<span>base <code>${esc(m.base)}</code></span>`);
  if(m.author) parts.push(`<span>${esc(m.author)}</span>`); if(m.date) parts.push(`<span>${esc(m.date)}</span>`);
  if(m.branch) parts.push(`<span>branch <code>${esc(m.branch)}</code></span>`);
  if(m.url) parts.push(`<a href="${esc(m.url)}" target="_blank">${esc(m.url.replace(/^https?:\/\//,''))}</a>`);
  parts.push(`<span class="hint">${esc(s.created?.slice(0,16).replace('T',' ')||'')}</span>`);
  parts.push(`<span class="hint" title="hudiff -s ${esc(s.id)} …">session <code>${esc(s.id)}</code></span>`);
  if(s.archived) parts.push(`<span class="chip" style="background:var(--bg3)">archived ${esc(String(s.archived).slice(0,10))}</span>`);
  const title = notes.title || s.title;
  const unsent = S.data.comments.filter(c=>c.status==='open').length;
  $('#hdr').innerHTML = `
    <div class="row">
      <a href="/" class="hint" title="All sessions" style="white-space:nowrap">☰ All sessions</a>
      <h1><a class="brand" href="/" title="All sessions"><svg class="mark" viewBox="0 0 64 64" width="20" height="20" fill="none" stroke-width="6.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21V9h12M43 9h12v12M9 43v12h12M55 43v12H43" style="stroke:var(--fg3)"/><path d="M22 26h15" style="stroke:var(--del-mark)"/><path d="M22 38h20" style="stroke:var(--add-mark)"/></svg></a><span class="kind ${esc(s.kind)}">${esc(kindCN[s.kind]||s.kind)}</span>${esc(title)}</h1>
      <div class="toolbar">
        <button class="ghost sm" id="archive" title="${s.archived?'unarchive; it shows on the home page again':'archive: hidden from the home page, data and link kept'}">${s.archived?'↩ Unarchive':'⌸ Archive'}</button>
        <label class="hint" style="display:flex;gap:4px;align-items:center"><input type="checkbox" id="showres" ${S.showResolved?'checked':''}> show resolved</label>
        <button class="ghost sm" id="theme"></button>
        ${unsent?`<span class="cnt">${unsent} unsent</span>`:''}
        <span class="sendgrp"><button class="primary" id="submit" ${unsent?'':'disabled'} title="Wake any agent waiting on this session">Send to agent</button><button class="drop" id="agents-btn" title="Choose which agent gets it">▾</button><span class="tgt" id="send-tgt"></span><div class="agpop" id="agentpop"></div></span>
      </div>
    </div>
    <div class="meta">${parts.join('')}</div>
    ${s.brief&&(s.brief.prompt||s.brief.presets?.length)?`<div class="brief"><span class="k">Review request</span><span class="v">${esc([s.brief.prompt, ...(s.brief.presets||[]).map(p=>p.text)].filter(Boolean).join('\n'))}</span></div>`:''}
    <div class="summary ${notes.summary?(S.sumOpen?'':'collapsed'):'empty'}" id="summary">${notes.summary?`<div class="md">${md(notes.summary)}</div><span class="tog">${S.sumOpen?'collapse ▴':'expand ▾'}</span>`:'the agent has not written the overview yet; this page refreshes on its own…'}</div>
    ${m.body?`<details class="body"><summary>raw ${s.kind==='pr'?'PR description':'commit message'}</summary><pre>${esc(m.body)}</pre></details>`:''}`;
  $('#showres').onchange = e => { S.showResolved = e.target.checked; renderComments(); renderNotes(); };
  $('#submit').onclick = () => doSubmit(AG.bound && AG.alive ? AG.bound.pid : 0);
  $('#agents-btn').onclick = e => { e.stopPropagation(); toggleAgentPop(); };
  $('#archive').onclick = async () => { const to=!s.archived; if(to && !(await dlg.confirm('Archive this session?', 'It disappears from the home page; the data and the link stay, and you can unarchive it any time.', {ok:'Archive'}))) return; await fetch('/api/s/'+encodeURIComponent(s.id),{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({archived:to})}); const d=await api('GET','/api/session'); S.data.session=d.session; renderHeader(); toast(to?'archived':'unarchived'); };
  renderSendTarget();
  $('#theme').onclick = () => applyTheme({auto:'light',light:'dark',dark:'auto'}[S.theme]);
  if(notes.summary) $('#summary').onclick = e => { if(e.target.closest('a')) return; S.sumOpen=!S.sumOpen; LS.set('sum.'+s.id, S.sumOpen); const el=$('#summary'); el.classList.toggle('collapsed', !S.sumOpen); el.querySelector('.tog').textContent = S.sumOpen?'collapse ▴':'expand ▾'; };
  applyTheme(S.theme);
}

const T = { q:'', exts:null, closed:new Set() };   // file tree: filter text / ticked extensions (null = all) / folded dirs
function extOf(path){ const b=path.split('/').pop(); return b.includes('.') ? '.'+b.split('.').pop().toLowerCase() : '(no extension)'; }
function renderTree(){
  const files=S.data.diff.files;
  const q=T.q.trim().toLowerCase();
  const visible = new Set(files.map((f,i)=>i).filter(i=>{ const f=files[i]; if(q && !f.path.toLowerCase().includes(q)) return false; if(T.exts && !T.exts.has(extOf(f.path))) return false; return true; }));
  const root = {dirs:{}, files:[]};
  files.forEach((f,i)=>{ if(!visible.has(i)) return; const parts=f.path.split('/'); let n=root; parts.slice(0,-1).forEach(p=>{ n=n.dirs[p]??=({dirs:{},files:[]}); }); n.files.push({f,i,name:parts.at(-1)}); });
  const squash=(n,name)=>{ while(Object.keys(n.dirs).length===1 && n.files.length===0){ const k=Object.keys(n.dirs)[0]; name=name+'/'+k; n=n.dirs[k]; } return [n,name]; };
  // Comment badge: unresolved → filled bubble with a count (amber when unsent, blue otherwise); all resolved → ticked bubble in pale green
  const cmark = fi => { const cs=S.data.comments.filter(c=>c.file===files[fi].path); if(!cs.length) return '';
    const live=cs.filter(c=>c.status!=='resolved'), open=live.filter(c=>c.status==='open').length;
    const tip=`${cs.length} comments: ${open} unsent / ${live.length-open} sent or replied / ${cs.length-live.length} resolved`;
    if(live.length) return `<span class="cm nf ${open?'open':'live'}" title="${tip}">${U(0xf075)}</span>`;
    return `<span class="cm nf done" title="${tip}">${U(0xf0e5)}</span>`; };
  let html='';
  const walk=(n,d,prefix,hidden)=>{
    n.files.sort((a,b)=>a.name.localeCompare(b.name)).forEach(({f,i,name})=>{
      html+=`<div class="file ${i===S.active?'active':''}" data-fi="${i}" style="--d:${d};${hidden?'display:none':''}" title="${esc(f.path)}">${icHTML(f.path)}<span class="name">${esc(name)}</span>${cmark(i)}<span class="pm"><span class="a">+${f.add}</span> <span class="d">−${f.del}</span></span>${gsHTML(f.status)}</div>`; });
    Object.keys(n.dirs).sort().forEach(k=>{ const [sub,name]=squash(n.dirs[k],k); const full=prefix?prefix+'/'+name:name; const closed=!q && T.closed.has(full);
      html+=`<div class="dir ${closed?'closed':''}" data-path="${esc(full)}" style="--d:${d};${hidden?'display:none':''}"><span class="chev">▾</span><span class="ic nf">${FOLDER}</span>${esc(name)}</div>`; walk(sub,d+1,full,hidden||closed); });
  };
  walk(root,0,'',false);
  if(!visible.size) html=`<div class="empty">no matching files</div>`;
  $('#tree-list').innerHTML = html;
  $('#tree-list').querySelectorAll('.file').forEach(el=>el.onclick=()=>{ showMid('code'); const fb=document.getElementById('f-'+el.dataset.fi); fb.classList.remove('collapsed'); scrollCodeTo(fb, 0); setActive(+el.dataset.fi); });
  $('#tree-list').querySelectorAll('.dir').forEach(el=>el.onclick=()=>{ const p=el.dataset.path; if(T.closed.has(p)) T.closed.delete(p); else T.closed.add(p); LS.set('closed.'+S.data.session.id, [...T.closed]); renderTree(); });
  // extension dropdown
  const counts={}; files.forEach(f=>{ const e=extOf(f.path); counts[e]=(counts[e]||0)+1; });
  const exts=Object.keys(counts).sort();
  $('#extf-list').innerHTML = `<label class="all"><input type="checkbox" id="ext-all" ${T.exts?'':'checked'}> all <span class="n">${files.length}</span></label>` + exts.map(e=>`<label><input type="checkbox" data-ext="${esc(e)}" ${!T.exts||T.exts.has(e)?'checked':''}> ${esc(e)} <span class="n">${counts[e]}</span></label>`).join('');
  $('#extf summary').classList.toggle('on', !!T.exts);
  $('#extf summary').textContent = T.exts ? `type ${T.exts.size}/${exts.length}` : 'type';
  $('#ext-all').onchange = () => { T.exts=null; renderTree(); };
  $('#extf-list').querySelectorAll('input[data-ext]').forEach(cb=>cb.onchange=()=>{ const on=new Set([...$('#extf-list').querySelectorAll('input[data-ext]:checked')].map(x=>x.dataset.ext)); T.exts = on.size===exts.length ? null : on; renderTree(); });
  $('#q-x').style.display = T.q ? '' : 'none';
  $('#fold-all').textContent = T.closed.size ? '⊞' : '⊟';
}
function setupTree(){
  $('#q-ic').textContent = U(0xf002);
  $('#q').oninput = e => { T.q=e.target.value; renderTree(); };
  $('#q').onkeydown = e => { if(e.key==='Escape'){ $('#q').value=''; T.q=''; renderTree(); $('#q').blur(); } e.stopPropagation(); };
  $('#q-x').onclick = () => { $('#q').value=''; T.q=''; renderTree(); };
  $('#fold-all').onclick = () => { if(T.closed.size){ T.closed.clear(); } else { document.querySelectorAll('#tree-list .dir').forEach(d=>T.closed.add(d.dataset.path)); } LS.set('closed.'+S.data.session.id, [...T.closed]); renderTree(); };
  $('#hide-nav').onclick = () => $('#tog-l').click();
  document.addEventListener('click', e => { const d=$('#extf'); if(d.open && !d.contains(e.target)) d.open=false; });
}

function renderDiff(){
  const notes = S.data.notes.files||{};
  let html='';
  S.data.diff.files.forEach((f,fi)=>{
    const fn = notes[f.path]||{}; const lang = langFor(f.path);
    const dir = f.path.includes('/') ? f.path.slice(0, f.path.lastIndexOf('/')+1) : ''; const base = f.path.slice(dir.length);
    const stcn = {A:'added',D:'Delete',R:'renamed',M:'modified'}[f.status]||f.status;
    html+=`<div class="file-block" id="f-${fi}" data-fi="${fi}"><div class="file-head" data-fi="${fi}"><span class="tri">▾</span><span class="st ${f.status}">${stcn}</span><span class="path"><span class="dir">${esc(dir)}</span>${esc(base)}${f.status==='R'?` <span class="hint">← ${esc(f.old_path)}</span>`:''}</span>${fn.summary?`<span class="fsum">· ${inline(fn.summary)}</span>`:'<span class="fsum"></span>'}<span class="pm"><b style="color:var(--add-fg)">+${f.add}</b> <b style="color:var(--del-fg)">−${f.del}</b></span></div>`;
    if(f.binary) html+=`<div class="hint binhint">binary file</div>`;
    f.hunks.forEach((h,hi)=>{
      html+=`<table class="hunk" id="h-${fi}-${hi}" data-fi="${fi}" data-hi="${hi}"><colgroup><col style="width:48px"><col style="width:48px"><col style="width:24px"><col></colgroup><tr class="hh" data-fi="${fi}" data-hi="${hi}"><td colspan="4"><span class="hno ${(()=>{const x=(notes[f.path]?.hunks||{})[String(hi)]; return x&&!x.skip?'noted':'';})()}" title="jump to the note for this hunk">#${hi}</span>${esc(h.header)}${(()=>{const x=(notes[f.path]?.hunks||{}); return (String(hi) in x) && (x[String(hi)]===null||x[String(hi)]?.skip) ? '<span class="skip">· needs no note</span>' : '';})()}<button class="sm hbtn" data-act="hunkc" data-fi="${fi}" data-hi="${hi}" title="comment on the whole hunk">💬 whole hunk</button></td></tr>`;
      h.lines.forEach(l=>{
        const cls = l.t==='+'?'add':l.t==='-'?'del':'ctx';
        const side = l.t==='-'?'old':'new'; const ln = side==='new'?l.n:l.o;
        html+=`<tr class="line ${cls}" data-fi="${fi}" data-hi="${hi}" data-side="${side}" data-line="${ln}"><td class="num">${l.o??''}</td><td class="num">${l.n??''}</td><td class="gut" title="comment on this line"></td><td class="code"><span class="m">${l.t===' '?'':l.t}</span>${hl(l.s, lang)}${l.nonl?'<span class="hint" title="no newline at end of file"> ⏎∅</span>':''}</td></tr>`;
      });
      html+='</table>';
    });
    html+='</div>';
  });
  $('#diff').innerHTML = html;
  $('#diff').onclick = e => {
    const b = e.target.closest('button');
    if(b){
      if(b.dataset.act==='hunkc'){ openForm({target:'hunk', fi:+b.dataset.fi, hi:+b.dataset.hi}, b.closest('tr')); return; }
      return;
    }
    const numCell = e.target.closest('td.num'); if(numCell){ const trr=numCell.closest('tr.line'); if(trr && trr.dataset.rv){ showMid('review'); const el=document.getElementById('rv-'+trr.dataset.rv); if(el){ el.scrollIntoView({block:'center'}); el.classList.remove('locate'); void el.offsetWidth; el.classList.add('locate'); } return; } }
    const gut = e.target.closest('td.gut');
    if(gut){ const tr=gut.closest('tr.line'); if(tr) openForm({target:'line', fi:+tr.dataset.fi, hi:+tr.dataset.hi, side:tr.dataset.side, line:+tr.dataset.line}, tr); return; }
    const fh = e.target.closest('.file-head'); if(fh){ fh.parentElement.classList.toggle('collapsed'); return; }
    // Reverse: clicking a line or a hunk header jumps to its note card (not while selecting text)
    const tr = e.target.closest('tr[data-fi]'); if(!tr) return;
    if(e.target.closest('.crow')) return;
    const fi=+tr.dataset.fi, hi=+tr.dataset.hi;
    if(e.target.closest('.hno')){ focusCard(fi, hi); return; }          // the #N chip always jumps
    if(!getSelection().isCollapsed) return;
    if(!tr.classList.contains('line')) return;
    // Only jump when the note actually covers this line; the same hunk outside its lines range does not count
    const r = noteRange(fi, hi);
    if(!r){ return; }
    if(!tr.classList.contains('noted')) return;
    focusCard(fi, hi);
  };
  markNoted(); applyPin();
  const pickActive = () => {
    if(Date.now() < (S.ioLock||0)) return;
    const pane=$('#diff'); const pr=pane.getBoundingClientRect(); const y=pr.top + pr.height*0.2 + 12;
    let hit=null;
    for(const t of document.querySelectorAll('table.hunk')){ const r=t.getBoundingClientRect(); if(r.bottom < pr.top || r.top > pr.bottom) continue; if(r.top <= y && r.bottom >= y){ hit=t; break; } if(!hit && r.top > y){ hit=t; break; } }
    if(hit) setActive(+hit.dataset.fi, +hit.dataset.hi);
  };
  let ioTimer=null; $('#diff').onscroll = () => { if(ioTimer) return; ioTimer=setTimeout(()=>{ ioTimer=null; pickActive(); }, 80); };
  const io = new IntersectionObserver(() => pickActive(), {rootMargin:'-10% 0px -70% 0px', threshold:0});
  document.querySelectorAll('table.hunk').forEach(t=>io.observe(t));
}

function setActive(fi, hi){
  const changed = fi!==S.active;
  S.active = fi; if(hi!==undefined) S.activeHunk = hi;
  if(changed){ $('#tree').querySelectorAll('.file').forEach(el=>el.classList.toggle('active', +el.dataset.fi===fi)); renderNotes(); }
  else document.querySelectorAll('#notes .card').forEach(c=>c.classList.toggle('active', +c.dataset.hi===S.activeHunk));
  const ac = document.querySelector('#notes .card.active'); if(ac) ac.scrollIntoView({block:'nearest'});
}

// ── Comments ──
function threadHTML(c){
  const stale = c.stale?`<span class="chip stale">⚠ line changed</span>`:'';
  const stcn = {open:'unsent',sent:'sent to the agent',replied:'agent replied',resolved:'resolved'}[c.status]||c.status;
  const adone = c.agent_done && c.status!=='resolved' ? `<span class="chip" style="background:var(--ok-bg);color:var(--ok)">✔ handled by the agent, awaiting you</span>` : '';
  const rootUnsent = c.status==='open' && !(c.replies||[]).some(r=>r.who==='user');
  const rootChip = c.status==='open' ? (rootUnsent?`<span class="chip open">unsent</span>`:`<span class="chip sent">sent</span>`) : `<span class="chip ${c.status}">${stcn}</span>`;
  let h = `<div class="thread" data-id="${c.id}"><div class="meta"><span class="av" style="background:${authorColor(c.author)}">${esc(initial(c.author))}</span><span class="who">${esc(who(c.author))}</span>${rootChip}${adone}${stale}<span>${esc(c.created?.slice(5,16).replace('T',' ')||'')}</span><span class="hint">${c.id}</span></div><div class="msg">${inline(c.body)}</div>`;
  (c.replies||[]).forEach(r=>{ const ag=r.who==='agent'; const rc = ag ? '' : (r.sent===false ? `<span class="chip open">unsent</span>` : (r.sent===true ? `<span class="chip sent">sent</span>` : '')); h+=`<div class="reply"><div class="meta"><span class="av ${ag?'agent':''}">${ag?'A':'me'}</span><span class="who ${r.who}">${ag?'agent':'me'}</span>${rc}<span>${esc(r.ts?.slice(5,16).replace('T',' ')||'')}</span></div><div class="msg">${ag?md(r.body):inline(r.body)}</div></div>`; });
  h += `<div class="acts"><button class="sm" data-act="reply">Reply</button>${c.status==='resolved'?'<button class="sm" data-act="reopen">reopen</button>':`<button class="sm ${c.agent_done?'primary':''}" data-act="resolve">${c.agent_done?'Confirm resolved':'Resolve'}</button>`}${(c.replies||[]).length?'':'<button class="sm" data-act="edit">Edit</button>'}<button class="sm ghost" data-act="del">Delete</button></div></div>`;
  return h;
}
function bindThreadActs(container){
  container.querySelectorAll('.thread .acts button').forEach(b=>b.onclick=async(e)=>{
    e.stopPropagation();
    const th=b.closest('.thread'), id=th.dataset.id, c=S.data.comments.find(x=>x.id===id);
    const act=b.dataset.act;
    if(act==='del'){ if(!(await dlg.confirm('Delete this comment?', 'The replies below go with it. This cannot be undone.', {ok:'Delete', danger:true}))) return; await api('DELETE','/api/comments/'+id); return refreshComments(); }
    if(act==='resolve'){ await api('PATCH','/api/comments/'+id,{status:'resolved'}); return refreshComments(); }
    if(act==='reopen'){ await api('PATCH','/api/comments/'+id,{status:'open'}); return refreshComments(); }
    if(act==='reply'){ if(th.querySelector('.cform')) return; const f=formEl('', async v=>{ await api('POST',`/api/comments/${id}/reply`,{who:'user',body:v}); await refreshComments(); }, ()=>f.remove()); th.appendChild(f); f.querySelector('textarea').focus(); return; }
    if(act==='edit'){ if(th.querySelector('.cform')) return; const f=formEl(c.body, async v=>{ await api('PATCH','/api/comments/'+id,{body:v}); await refreshComments(); }, ()=>f.remove()); th.appendChild(f); f.querySelector('textarea').focus(); return; }
  });
}
function formEl(initial, onSave, onCancel){
  const d=document.createElement('div'); d.className='cform';
  d.innerHTML=`<textarea placeholder="Write a comment… (⌘/Ctrl+Enter to save, Esc to cancel)">${esc(initial)}</textarea><div class="acts"><button class="primary sm" data-a="save">Save</button><button class="sm" data-a="cancel">Cancel</button><span>saved — hit “Send to agent” at the top right to notify the agent</span></div>`;
  const ta=d.querySelector('textarea');
  const save=async()=>{ const v=ta.value.trim(); if(!v) return toast('empty'); await onSave(v); };
  d.querySelector('[data-a=save]').onclick=e=>{e.stopPropagation();save();}; d.querySelector('[data-a=cancel]').onclick=e=>{e.stopPropagation();onCancel();};
  ta.onkeydown=e=>{ if((e.metaKey||e.ctrlKey)&&e.key==='Enter'){e.preventDefault();save();} if(e.key==='Escape'){onCancel();} e.stopPropagation(); };
  d.onclick=e=>e.stopPropagation();
  return d;
}
function openForm(anchor, afterTr){
  const key=JSON.stringify(anchor); if(S.openForms.has(key)) return;
  const f = S.data.diff.files[anchor.fi];
  const tr=document.createElement('tr'); tr.className='crow form'; tr.dataset.key=key;
  const td=document.createElement('td'); td.colSpan=4; tr.appendChild(td);
  const el=formEl('', async v=>{
    await api('POST','/api/comments',{target:anchor.target, file:f.path, hunk:anchor.hi, side:anchor.side, line:anchor.line, body:v});
    S.openForms.delete(key); tr.remove(); await refreshComments(); toast('saved');
  }, ()=>{ S.openForms.delete(key); tr.remove(); });
  td.appendChild(el); S.openForms.add(key);
  let at=afterTr; while(at.nextElementSibling && at.nextElementSibling.classList.contains('crow') && !at.nextElementSibling.classList.contains('form')) at=at.nextElementSibling;
  at.after(tr); el.querySelector('textarea').focus();
}
function renderComments(){
  document.querySelectorAll('tr.crow:not(.form)').forEach(e=>e.remove());
  document.querySelectorAll('tr.line.has-c').forEach(e=>e.classList.remove('has-c'));
  const byKey = {};
  S.data.comments.forEach(c=>{
    if(!S.showResolved && c.status==='resolved') return;
    if(c.target==='general') return;
    const fi = S.data.diff.files.findIndex(f=>f.path===c.file); if(fi<0) return;
    const key = c.target==='line' ? `L|${fi}|${c.hunk}|${c.side}|${c.line}` : `H|${fi}|${c.hunk}`;
    (byKey[key]??=[]).push(c);
  });
  Object.entries(byKey).forEach(([key,cs])=>{
    const p=key.split('|'); let anchor;
    if(p[0]==='L') anchor=document.querySelector(`tr.line[data-fi="${p[1]}"][data-hi="${p[2]}"][data-side="${p[3]}"][data-line="${p[4]}"]`);
    else anchor=document.querySelector(`tr.hh[data-fi="${p[1]}"][data-hi="${p[2]}"]`);
    if(!anchor) return;
    anchor.classList.add('has-c');
    const tr=document.createElement('tr'); tr.className='crow'; const td=document.createElement('td'); td.colSpan=4; td.innerHTML=cs.map(threadHTML).join(''); tr.appendChild(td);
    anchor.after(tr); bindThreadActs(td);
  });
  const unsent = S.data.comments.filter(c=>c.status==='open').length;
  const cnt=$('#hdr .cnt'); const sub=$('#submit');
  if(unsent){ if(!cnt){ const s=document.createElement('span'); s.className='cnt'; sub.before(s); } $('#hdr .cnt').textContent=`${unsent} unsent`; } else if(cnt) cnt.remove();
  sub.disabled=!unsent;
  renderTree(); renderMidbar();
}
async function refreshComments(){ const d = await api('GET','/api/session'); S.data.comments=d.comments; S.state=d.state; renderComments(); renderNotes(); }

// ── Right pane: notes ──
function renderNotes(){
  const f = S.data.diff.files[S.active]; if(!f){ $('#notes').innerHTML=''; return; }
  const fn = (S.data.notes.files||{})[f.path]||{};
  const gen = S.data.comments.filter(c=>c.target==='general' && (S.showResolved||c.status!=='resolved'));
  const live = S.data.comments.filter(c=>c.status!=='resolved').length;
  const tabs = `<div class="tabs"><button class="tab ${S.tab==='notes'?'on':''}" data-tab="notes">Notes</button><button class="tab ${S.tab==='comments'?'on':''}" data-tab="comments">Comments<span class="n">${live}</span></button><span class="sp"></span><button class="ghost sm" id="hide-aside" title="collapse the right pane  ]">◨</button></div>`;
  if(S.tab==='comments'){ renderCommentList(tabs); return; }
  if(S.mid==='diagrams'){ renderDiagramList(tabs.replace('>Notes<', '>Diagrams<')); return; }
  if(S.mid==='review'){ renderReviewOutline(tabs.replace('>Notes<', '>Review items<')); return; }
  let html = tabs + `<div class="gen"><h3>overall</h3>${gen.map(threadHTML).join('')}<button class="sm" id="gen-add">+ comment on the whole change</button></div>`;
  html += `<h3>Notes · <span class="fname">${esc(f.path.split('/').pop())}</span></h3>`;
  if(fn.summary) html+=`<div class="fsum-box">${inline(fn.summary)}</div>`;
  const hunks = fn.hunks||{};
  const realCards = Object.values(hunks).filter(x=>x && !x.skip).length;
  if(!Object.keys(hunks).length) html+=`<div class="card pending">the agent has not written notes for this file yet…</div>`;
  else if(!realCards) html+=`<div class="card pending">the change in this file is mechanical; the agent wrote no separate note</div>`;
  f.hunks.forEach((h,hi)=>{
    const n = hunks[String(hi)]; if(!n || n.skip) return;
    const isOld=/^old/i.test(n.lines||'');
    const rng = n.lines ? `<span class="chip rng ${isOld?'old':''}">${isOld?'old':'new'} L${esc(String(n.lines).replace(/^(old|new):/i,''))}</span>` : `<span class="chip rng">new L${h.new_start}–${h.new_start+Math.max(h.new_count-1,0)}</span>`;
    const hc = S.data.comments.filter(c=>c.target==='hunk'&&c.file===f.path&&c.hunk===hi&&(S.showResolved||c.status!=='resolved'));
    html+=`<div class="card ${hi===S.activeHunk?'active':''}" data-hi="${hi}" data-lines="${esc(n.lines||'')}">
      <div class="head"><span class="hno">#${hi}</span>${n.title?`<span class="t">${inline(n.title)}</span>`:`<span class="t hint" style="font-weight:400">${esc(h.section||'')}</span>`}</div>
      <div class="chips">${rng}${(n.tags||[]).map(t=>`<span class="chip ${tagClass(t)}">${esc(t)}</span>`).join('')}${dgForHunk(f.path,hi).map(d=>`<span class="chip dgl" data-dg="${esc(d.id)}" title="${esc(d.title)}">${U(0xf1c5)} Diagrams</span>`).join('')}</div>
      <div class="body">${md(n.body)}</div>
      ${hc.length?`<div class="hc">${hc.map(threadHTML).join('')}</div>`:''}
      <div class="acts"><button class="sm ghost" data-act="hunkc">💬 comment on this note</button></div></div>`;
  });
  html += `<div class="hint" style="margin-top:14px;line-height:1.9">click a card → jump to the code and pin it (<kbd>Esc</kbd> to unpin) · click a blue-edged line or <b>#N</b> → back to the card · <kbd>v</kbd> code/diagrams/review · <kbd>/</kbd> filter files · <kbd>n</kbd> / <kbd>p</kbd> previous/next hunk · <kbd>[</kbd> / <kbd>]</kbd> collapse the side panes (drag the sash to resize) · at a line start <kbd>+</kbd> comments that line · click a file header to fold it</div>`;
  $('#notes').innerHTML = html;
  bindThreadActs($('#notes'));
  bindTabs();
  applyPin();
  $('#gen-add').onclick = () => { if($('#notes .gen .cform')) return; const fm=formEl('', async v=>{ await api('POST','/api/comments',{target:'general',body:v}); await refreshComments(); }, ()=>fm.remove()); $('#gen-add').before(fm); fm.querySelector('textarea').focus(); };
  $('#notes').querySelectorAll('.card[data-hi]').forEach(card=>{
    const hi=+card.dataset.hi;
    card.onclick = e => { if(e.target.closest('button')||e.target.closest('.cform')||e.target.closest('.thread')) return; showMid('code'); const fb=document.getElementById('f-'+S.active); fb.classList.remove('collapsed'); S.pin={fi:S.active,hi}; applyPin();
      // With a lines range, scroll to its first line: in a big hunk that can be far from the header
      const t=document.getElementById(`h-${S.active}-${hi}`); const first=t && t.querySelector('tr.line.hl-pin'); const r=noteRange(S.active,hi);
      scrollCodeTo((first && r && !r.all) ? first : t, (first && r && !r.all) ? 0.2 : 0.05); };
    card.querySelector('[data-act=hunkc]').onclick = e => { e.stopPropagation(); showMid('code'); const hh=document.querySelector(`tr.hh[data-fi="${S.active}"][data-hi="${hi}"]`); scrollCodeTo(hh, 0.2); openForm({target:'hunk', fi:S.active, hi}, hh); };
    card.querySelectorAll('.dgl').forEach(c=>c.onclick=e=>{ e.stopPropagation(); scrollToDiagram(c.dataset.dg); });
    card.onmouseenter = () => highlightLines(S.active, hi, card.dataset.lines, true);
    card.onmouseleave = () => highlightLines(S.active, hi, card.dataset.lines, false);
  });
}
// Which lines a hunk note covers: its lines range if it has one, otherwise every +/- line in the hunk
function noteRange(fi, hi){
  const f=S.data.diff.files[fi]; const n=((S.data.notes.files||{})[f?.path]||{}).hunks?.[String(hi)]; if(!n||n.skip) return null;
  const m=/^(?:(old|new):)?L?(\d+)(?:\s*[-–]\s*L?(\d+))?/i.exec(n.lines||'');
  if(m) return {side:(m[1]||'new').toLowerCase(), a:+m[2], b:+(m[3]||m[2]), all:false};
  const h=f.hunks[hi]; return {side:'new', a:h.new_start, b:h.new_start+Math.max(h.new_count-1,0), all:true};
}
function lineInRange(tr, r){ return r.all ? (tr.classList.contains('add')||tr.classList.contains('del')) : (tr.dataset.side===r.side && +tr.dataset.line>=r.a && +tr.dataset.line<=r.b); }
function markNoted(){
  document.querySelectorAll('table.hunk').forEach(t=>{ const r=noteRange(+t.dataset.fi,+t.dataset.hi); t.querySelectorAll('tr.line').forEach(tr=>tr.classList.toggle('noted', !!r && lineInRange(tr,r))); });
}
function applyPin(){
  document.querySelectorAll('tr.line.hl-pin').forEach(t=>t.classList.remove('hl-pin'));
  document.querySelectorAll('#notes .card.pinned').forEach(c=>c.classList.remove('pinned'));
  if(!S.pin) return;
  const {fi,hi}=S.pin; const f=S.data.diff.files[fi]; const n=((S.data.notes.files||{})[f?.path]||{}).hunks?.[String(hi)];
  const t=document.getElementById(`h-${fi}-${hi}`); if(!t) return;
  const r=noteRange(fi,hi); t.querySelectorAll('tr.line').forEach(tr=>tr.classList.toggle('hl-pin', !!r && lineInRange(tr,r)));
  if(fi===S.active){ const c=document.querySelector(`#notes .card[data-hi="${hi}"]`); if(c) c.classList.add('pinned'); }
}
function pin(fi,hi){ S.pin = (S.pin && S.pin.fi===fi && S.pin.hi===hi) ? null : {fi,hi}; applyPin(); }
function focusCard(fi,hi){
  const f=S.data.diff.files[fi]; const has=((S.data.notes.files||{})[f.path]||{}).hunks?.[String(hi)];
  setActive(fi,hi);
  if(!has||has.skip) return;
  S.pin={fi,hi}; applyPin();
  const c=document.querySelector(`#notes .card[data-hi="${hi}"]`); if(!c) return;
  if($('#main').classList.contains('aside-off')) $('#tog-r').click();
  c.scrollIntoView({block:'center'}); c.classList.remove('flash'); void c.offsetWidth; c.classList.add('flash');
}
const isDark = () => S.theme==='dark' || (S.theme==='auto' && matchMedia('(prefers-color-scheme: dark)').matches);
const dgSrc = (dg, board) => `${BASE}/api/diagram?i=${encodeURIComponent(dg.id)}&theme=${isDark()?'dark':'light'}${board===undefined?'':'&board='+board}&v=${S.state?.notes_v||0}-${S.state?.diff_v||0}`;
const hunkLabel = ref => { const m=/^(.+)#(\d+)$/.exec(ref); return m ? m[1].split('/').pop()+'#'+m[2] : ref; };
function dgForHunk(path, hi){ return (S.data.diagrams||[]).filter(d=>(d.hunks||[]).includes(`${path}#${hi}`)); }

const PRIO_CN={high:'high',medium:'medium',low:'low'};
function rvCounts(){ const c={high:0,medium:0,low:0,open:0,done:0,wontfix:0}; ((S.data.review||{}).items||[]).forEach(it=>{ const st=it.status||'open'; c[st]=(c[st]||0)+1; if(st==='open') c[it.priority]=(c[it.priority]||0)+1; }); return c; }
function renderMidbar(){
  const n=(S.data.diagrams||[]).length; const rc=rvCounts(); const hasRv=!!S.data.review;
  const cs=S.data.comments||[]; const cDone=cs.filter(c=>c.status==='resolved').length; const rTotal=((S.data.review||{}).items||[]).length; const rDone=rc.done+rc.wontfix;
  const badge=(done,total,red)=>`<span class="n" style="${total&&done===total?'background:var(--ok);color:#fff':(red?'background:var(--del-fg);color:#fff':'')}">${done}<span style="opacity:.6">/${total}</span></span>`;
  $('#midbar').innerHTML = `<button class="tab ${S.mid==='code'?'on':''}" data-mid="code" title="${cs.length} comments, ${cDone} resolved">Code${cs.length?badge(cDone,cs.length,false):''}</button><button class="tab ${S.mid==='diagrams'?'on':''}" data-mid="diagrams">Diagrams<span class="n">${n}</span></button><button class="tab ${S.mid==='review'?'on':''}" data-mid="review" title="${hasRv?`${rDone} of ${rTotal} handled, ${rc.open} open`:'no review report yet'}">Review${hasRv?badge(rDone,rTotal,rc.high>0):''}</button><span class="sp"></span>${S.mid==='diagrams'?`<span class="hint">click a diagram to zoom · click a hunk chip to jump back to the code</span>`:''}${S.mid==='review'&&hasRv?`<span class="hint">tick = handled · click file:line to jump to the code · agents use hudiff review done &lt;id&gt;</span>`:''}`;
  $('#midbar').querySelectorAll('.tab').forEach(b=>b.onclick=()=>showMid(b.dataset.mid));
  $('#diff').style.display = S.mid==='code'?'':'none';
  $('#diagrams').style.display = S.mid==='diagrams'?'':'none';
  $('#review').style.display = S.mid==='review'?'':'none';
}
function rvLoc(it){ if(!it.file) return ''; const f=it.file, dir=f.includes('/')?f.slice(0,f.lastIndexOf('/')+1):'', base=f.slice(dir.length); return `<span class="loc" data-id="${esc(it.id)}"><span class="d">${esc(dir)}</span>${esc(base)}${it.line?` <b>${it.side==='old'?'old ':''}L${it.line}</b>`:''}${it.hunk!=null?` <span class="d">#${it.hunk}</span>`:''}</span>`; }
function mdDiffColor(html){ return html.replace(/<pre><code>([\s\S]*?)<\/code><\/pre>/g,(m,body)=>'<pre><code>'+body.split('\n').map(l=>l.startsWith('-')?`<span class="dl">${l}</span>`:l.startsWith('+')?`<span class="ad">${l}</span>`:l).join('\n')+'</code></pre>'); }
function renderReview(){
  const rv=S.data.review;
  if(!rv){ renderReviewRequest($('#review'), true); return; }
  const c=rvCounts(); const items=rv.items||[];
  const rounds=rv.rounds||[];
  let html=`<div class="rv-head"><h2>Review report</h2><span class="chip">${esc(rv.author||'reviewer')}</span>${rounds.length>1?`<span class="chip">${rounds.length} rounds</span>`:''}<span class="hint">${esc((rv.updated||'').slice(0,16).replace('T',' '))} · ${c.open} open · ${c.done} handled · ${c.wontfix} wontfix</span><button class="sm" id="rv-again">↻ Review again</button></div><div id="rv-req" style="display:none"></div>`;
  if(rounds.length>1){ html+=`<div class="rv-sum">${rounds.map(r=>`<div class="rnd"><span class="chip">round ${r.round}</span> <span class="hint">${esc(r.author||'')} ${esc(String(r.at||'').slice(5,16).replace('T',' '))} · added ${r.added||0} items</span><div>${md(r.summary||'')}</div></div>`).join('')}</div>`; }
  else if(rv.summary) html+=`<div class="rv-sum">${md(rv.summary)}</div>`;
  if((rv.strengths||[]).length) html+=`<h3 style="font-size:12px;color:var(--fg2);margin:0 0 4px">Strengths</h3><ul class="rv-good">${rv.strengths.map(x=>`<li>${inline(x)}</li>`).join('')}</ul>`;
  ['high','medium','low'].forEach(pr=>{
    const its=items.filter(it=>it.priority===pr); if(!its.length) return;
    const open=its.filter(it=>(it.status||'open')==='open').length;
    html+=`<div class="rv-sec"><span class="pr ${pr}">${PRIO_CN[pr]}</span><h3>${pr==='high'?'Blocking — must be fixed':pr==='medium'?'Worth fixing — not blocking':'Style and readability — optional'}</h3><span class="cnt">${open}/${its.length} open</span></div>`;
    its.forEach(it=>{ const st=it.status||'open';
      html+=`<div class="rvi ${pr} ${st}" id="rv-${esc(it.id)}" data-id="${esc(it.id)}">
        <div class="cb" title="${st==='open'?'Mark handled':'reopen'}">${st==='done'?'✓':st==='wontfix'?'–':''}</div>
        <div class="t">${inline(it.title)}<span class="id">${esc(it.id)}</span>${rounds.length>1&&it.round?`<span class="chip" title="raised in round ${it.round}">R${it.round}</span>`:''}${st==='wontfix'?'<span class="chip" style="background:var(--del-bg);color:var(--del-fg)">wontfix</span>':''}</div>
        ${rvLoc(it)}
        ${it.snippet?`<div class="snip">${esc(it.snippet)}</div>`:''}
        ${it.body?`<div class="body">${md(it.body)}</div>`:''}
        ${it.suggestion?`<div class="sug"><span class="k">Suggested fix</span>${mdDiffColor(md(it.suggestion))}</div>`:''}
        ${it.resolution?`<div class="res">${st==='wontfix'?'Reason':'Resolution'}(${esc(it.resolved_by||'')}): ${inline(it.resolution)}</div>`:''}
        ${(it.notes||[]).length?`<div class="notes">${it.notes.map(n=>`<div class="note ${esc(n.verdict||'info')}"><span class="v">${n.verdict==='ok'?'✓ re-review passed':n.verdict==='disagree'?'⚠ disagrees':'· note'}</span><span class="w">${esc(n.who||'')} · R${n.round||'?'}</span><div class="b">${md(n.body)}</div></div>`).join('')}</div>`:''}
        <div class="acts">${st==='open'?`<button class="sm" data-act="done">handled</button><button class="sm reject" data-act="wontfix">wontfix</button>`:`<button class="sm ghost" data-act="reopen">reopen</button>`}<button class="sm ghost" data-act="note">note</button></div>
      </div>`;
    });
  });
  $('#review').innerHTML=html;
  $('#rv-again').onclick=()=>{ const box=$('#rv-req'); if(box.style.display==='none'){ box.style.display=''; renderReviewRequest(box, false); } else box.style.display='none'; };
  const act=async(id,st)=>{ let text=''; if(st!=='open'){ text=await dlg.prompt(st==='done'?'Mark handled':'Mark this wontfix', st==='done'?'One line on what you did (optional):':'One line on why not (optional):', {ok:st==='done'?'handled':'wontfix', danger:st==='wontfix', placeholder:'optional'}); if(text===null) return; } await api('PATCH','/api/review/'+id,{status:st,resolution:text,by:'user'}); await refreshReview(); };
  $('#review').querySelectorAll('.rvi').forEach(el=>{
    const id=el.dataset.id; const it=items.find(x=>x.id===id);
    el.querySelector('.cb').onclick=()=>act(id,(it.status||'open')==='open'?'done':'open');
    el.querySelectorAll('.acts button').forEach(b=>b.onclick=async()=>{ if(b.dataset.act==='note'){ const t=await dlg.prompt('note', 'A note for the re-reviewer or for yourself; it shows under this item:', {ok:'Add', placeholder:'e.g. wontfix because…'}); if(!t||!t.trim()) return; await api('POST',`/api/review/${id}/note`,{body:t,who:'user',verdict:'info'}); await refreshReview(); return; } act(id,{done:'done',wontfix:'wontfix',reopen:'open'}[b.dataset.act]); });
    const loc=el.querySelector('.loc'); if(loc) loc.onclick=()=>jumpToReviewItem(id);
  });
  const io=new IntersectionObserver(es=>{ es.forEach(en=>{ if(en.isIntersecting){ document.querySelectorAll('.rvo').forEach(o=>o.classList.toggle('active', o.dataset.id===en.target.dataset.id)); } }); }, {root:$('#review'), rootMargin:'-10% 0px -70% 0px', threshold:0});
  document.querySelectorAll('.rvi').forEach(f=>io.observe(f));
}
async function refreshReview(){ const d=await api('GET','/api/session'); S.data.review=d.review; S.state=d.state; renderMidbar(); renderReview(); markReviewLines(); if(S.mid==='review') renderNotes(); }
// Request review: hudiff cannot reach another agent any more (tmux is gone), so
// it hands over the command and fires config.hooks.on_review_request.
async function renderReviewRequest(box, empty){
  const sid=S.data.session.id, cmd=`/hudiff review ${sid}`;
  box.innerHTML=`<div class="rvreq"><div class="rvreq-h">${empty?'<h2>No review report yet</h2>':''}<p>Ask a <b>different</b> agent than the one writing the notes to cross-review: it re-reads the change, the notes and the diagrams, then writes an overall verdict, the strengths, and high / medium / low items. This page refreshes when the report lands.</p></div><div class="aglist"><div class="agempty">Looking for agents…</div></div><div class="rvreq-cmd"><code>${esc(cmd)}</code><button class="sm" id="rvcopy">Copy</button></div></div>`;
  box.querySelector('#rvcopy').onclick=()=>{ copyText(cmd); };
  await loadAgents(true);
  const list=box.querySelector('.aglist');
  list.innerHTML=agentGroups();
  bindAgentRows(list, async (pid, name) => {
    const prev=S.data.review, n=((prev||{}).items||[]).length;
    const ok=await dlg.confirm(`Ask ${name} to review?`,
      `It gets «${cmd}» and re-reads the change, the notes, the diagrams and the comments${n?`, plus the ${n} item(s) from the last round and how you resolved them — a new round merges in, it does not overwrite`:''}. Whatever it is doing right now gets interrupted.`,
      {ok:'Send the review request'});
    if(!ok) return;
    const r=await api('POST','/api/review-request',{pid});
    if(r.agent){ AG.bound={pid:r.agent.pid,name:r.agent.name,kind:r.agent.kind}; AG.alive=true; renderSendTarget(); }
    await handOver(pid, cmd);
  });
}
function jumpToReviewItem(id){
  const it=((S.data.review||{}).items||[]).find(x=>x.id===id); if(!it||!it.file) return;
  const fi=S.data.diff.files.findIndex(f=>f.path===it.file); if(fi<0){ toast('the file is not in this diff'); return; }
  showMid('code'); const fb=document.getElementById('f-'+fi); fb.classList.remove('collapsed');
  let tr = it.line!=null ? document.querySelector(`tr.line[data-fi="${fi}"][data-hi="${it.hunk}"][data-side="${it.side||'new'}"][data-line="${it.line}"]`) : null;
  const target = tr || document.getElementById(`h-${fi}-${it.hunk??0}`) || fb;
  scrollCodeTo(target, 0.2); setActive(fi, it.hunk??0);
  if(tr){ tr.classList.add('hl'); setTimeout(()=>tr.classList.remove('hl'), 1800); }
}
function markReviewLines(){
  document.querySelectorAll('tr.line.rv-high,tr.line.rv-medium,tr.line.rv-low').forEach(t=>{ t.classList.remove('rv-high','rv-medium','rv-low'); t.removeAttribute('data-rv'); t.removeAttribute('title'); });
  ((S.data.review||{}).items||[]).forEach(it=>{ if(!it.file||it.line==null||(it.status||'open')!=='open') return; const fi=S.data.diff.files.findIndex(f=>f.path===it.file); if(fi<0) return;
    const tr=document.querySelector(`tr.line[data-fi="${fi}"][data-hi="${it.hunk}"][data-side="${it.side||'new'}"][data-line="${it.line}"]`); if(!tr) return;
    tr.classList.add('rv-'+it.priority); tr.dataset.rv=it.id; tr.title=`[${PRIO_CN[it.priority]}] ${it.title} — click a line number for its review item`; });
}
function renderReviewOutline(tabs){
  const rv=S.data.review; const c=rvCounts();
  let html=tabs+`<h3>Review items · ${c.open} open</h3>`;
  if(!rv){ html+=`<div class="hint">no review report yet</div>`; $('#notes').innerHTML=html; bindTabs(); return; }
  html+=`<div class="rv-stats"><span class="chip pr high">high ${c.high}</span><span class="chip pr medium">medium ${c.medium}</span><span class="chip pr low">low ${c.low}</span><span class="chip">${c.done} handled</span><span class="chip">${c.wontfix} wontfix</span></div>`;
  (rv.items||[]).forEach(it=>{ const dis=(it.notes||[]).some(n=>n.verdict==='disagree'); html+=`<div class="rvo ${it.priority} ${it.status||'open'}" data-id="${esc(it.id)}">${dis?'<span title="re-review disagrees" style="color:#bc4c00">⚠</span>':''}<span class="t">${esc(it.title)}</span><span class="l">${it.file?esc(it.file.split('/').pop())+(it.line?':'+it.line:''):''}</span></div>`; });
  html+=`<div class="hint" style="margin-top:14px;line-height:1.9">click an item to find it in the report · in the report click file:line to reach the code · a red/amber/blue edge left of a line number means a review item</div>`;
  $('#notes').innerHTML=html; bindTabs();
  $('#notes').querySelectorAll('.rvo').forEach(o=>o.onclick=()=>{ const el=document.getElementById('rv-'+o.dataset.id); if(el){ el.scrollIntoView({block:'center'}); el.classList.remove('locate'); void el.offsetWidth; el.classList.add('locate'); } });
}
function showMid(m){ if(S.mid===m) return; S.mid=m; LS.set('mid',m); renderMidbar(); renderNotes(); }

function renderDiagrams(){
  const dgs=S.data.diagrams||[];
  if(!dgs.length){ $('#diagrams').innerHTML = `<div class="hint" style="text-align:center;padding:60px 0">No diagrams yet. the agent attaches sequence / flow / class diagrams with its notes; the change map appears on its own when the diff touches symbols.</div>`; return; }
  let html='';
  dgs.forEach(dg=>{
    const chips=(dg.hunks||[]).map(r=>`<span class="chip hk" data-ref="${esc(r)}" title="${esc(r)}">${esc(hunkLabel(r))}</span>`).join('');
    // steps / layers / scenarios: one static board per SVG, stacked, each labelled.
    // They used to be one animated SVG flipping between near-identical frames.
    const boards = dg.boards && dg.boards.length>1 ? dg.boards : null;
    const body = boards
      ? `<div class="svgwrap boards">${boards.map((n,i)=>`<div class="board"><div class="bl">${esc(n)}</div><img src="${dgSrc(dg,i)}" alt="${esc(dg.title)} ${esc(n)}"></div>`).join('')}</div>`
      : `<div class="svgwrap"><img src="${dgSrc(dg)}" alt="${esc(dg.title)}"></div>`;
    html+=`<figure class="dg ${dg.id===S.activeDg?'active':''}" id="dg-${esc(dg.id)}" data-id="${esc(dg.id)}" style="margin:0 0 18px">
      <div class="dgh"><span class="t">${esc(dg.title)}</span><span class="kind">${esc(dg.kind)}${dg.auto?' · auto':''}</span>${chips}${dg.src?`<button class="sm ghost" data-act="src">Source</button>`:''}<button class="sm ghost" data-act="full">⛶ Zoom</button></div>
      ${body}
      ${dg.caption?`<figcaption>${md(dg.caption)}</figcaption>`:''}
      ${dg.src?`<pre class="src" style="display:none">${esc(dg.src)}</pre>`:''}</figure>`;
  });
  $('#diagrams').innerHTML = html;
  $('#diagrams').querySelectorAll('.hk').forEach(c=>c.onclick=()=>{ const m=/^(.+)#(\d+)$/.exec(c.dataset.ref); if(!m) return; const fi=S.data.diff.files.findIndex(f=>f.path===m[1]); if(fi<0) return; showMid('code'); const fb=document.getElementById('f-'+fi); fb.classList.remove('collapsed'); scrollCodeTo(document.getElementById(`h-${fi}-${m[2]}`), 0.05); setActive(fi,+m[2]); S.pin={fi,hi:+m[2]}; applyPin(); });
  $('#diagrams').querySelectorAll('[data-act=src]').forEach(b=>b.onclick=()=>{ const pre=b.closest('.dg').querySelector('pre.src'); pre.style.display = pre.style.display==='none'?'':'none'; });
  $('#diagrams').querySelectorAll('[data-act=full], .svgwrap').forEach(el=>el.onclick=e=>{ if(e.target.closest('button')&&el.tagName!=='BUTTON') return; const fig=el.closest('.dg'); const img=fig.querySelector('img'); const svg=fig.querySelector('svg'); openLightbox(fig.querySelector('.t').textContent, img?img.src:(svg?'data:image/svg+xml;charset=utf-8,'+encodeURIComponent(svg.outerHTML):null)); });
  const io=new IntersectionObserver(es=>{ es.forEach(en=>{ if(en.isIntersecting){ S.activeDg=en.target.dataset.id; document.querySelectorAll('.dg').forEach(f=>f.classList.toggle('active', f.dataset.id===S.activeDg)); document.querySelectorAll('.dgcard').forEach(c=>c.classList.toggle('active', c.dataset.id===S.activeDg)); } }); }, {root:$('#diagrams'), rootMargin:'-20% 0px -60% 0px', threshold:0});
  document.querySelectorAll('.dg').forEach(f=>io.observe(f));
}
function scrollToDiagram(id){ showMid('diagrams'); const f=document.getElementById('dg-'+id); if(f){ f.scrollIntoView({block:'start'}); S.activeDg=id; document.querySelectorAll('.dg').forEach(x=>x.classList.toggle('active', x.dataset.id===id)); } }

// Lightbox: wheel to zoom, drag to pan
const LB={scale:1,x:0,y:0,drag:null};
function lbApply(){ $('#lb-img').style.transform=`translate(${LB.x}px,${LB.y}px) scale(${LB.scale})`; }
function openLightbox(title, src){
  if(!src) return; $('#lb-title').textContent=title; const img=$('#lb-img'); img.src=src; $('#lb').style.display='flex';
  img.onload=()=>{ const body=$('#lb-body').getBoundingClientRect(); const sc=Math.min((body.width-40)/img.naturalWidth,(body.height-40)/img.naturalHeight,2); LB.scale=sc>0?sc:1; LB.x=(body.width-img.naturalWidth*LB.scale)/2; LB.y=(body.height-img.naturalHeight*LB.scale)/2; lbApply(); };
}
function closeLightbox(){ $('#lb').style.display='none'; $('#lb-img').src=''; }
$('#lb-x').onclick=closeLightbox;
$('#lb').onclick=e=>{ if(e.target.id==='lb'||e.target.id==='lb-body') closeLightbox(); };
$('#lb-body').onwheel=e=>{ e.preventDefault(); const r=$('#lb-body').getBoundingClientRect(); const mx=e.clientX-r.left, my=e.clientY-r.top; const k=e.deltaY<0?1.12:1/1.12; const ns=Math.max(.1,Math.min(8,LB.scale*k)); LB.x=mx-(mx-LB.x)*(ns/LB.scale); LB.y=my-(my-LB.y)*(ns/LB.scale); LB.scale=ns; lbApply(); };
$('#lb-body').onmousedown=e=>{ if(e.target.id==='lb-body'&&e.detail===1){} LB.drag={x:e.clientX-LB.x,y:e.clientY-LB.y}; $('#lb-body').classList.add('drag'); e.preventDefault(); };
document.addEventListener('mousemove',e=>{ if(!LB.drag) return; LB.x=e.clientX-LB.drag.x; LB.y=e.clientY-LB.drag.y; lbApply(); });
document.addEventListener('mouseup',()=>{ LB.drag=null; $('#lb-body').classList.remove('drag'); });

function renderDiagramList(tabs){
  const dgs=S.data.diagrams||[];
  let html = tabs + `<h3>Diagrams · ${dgs.length}</h3>`;
  if(!dgs.length) html+=`<div class="hint">no diagrams yet</div>`;
  dgs.forEach(dg=>{ html+=`<div class="dgcard ${dg.id===S.activeDg?'active':''}" data-id="${esc(dg.id)}"><div class="th"><img src="${dgSrc(dg, 0)}" alt=""></div><div class="tt"><span>${esc(dg.title)}</span><span class="hint">${dg.auto?'auto':(dg.hunks||[]).length+' hunks'}</span></div></div>`; });
  html += `<div class="hint" style="margin-top:14px;line-height:1.9">click a thumbnail to scroll to it · a hunk chip on the diagram jumps back to the code · <kbd>v</kbd> switches code/diagrams</div>`;
  $('#notes').innerHTML = html;
  bindTabs();
  $('#notes').querySelectorAll('.dgcard').forEach(c=>c.onclick=()=>scrollToDiagram(c.dataset.id));
}

// ── Send: the session is the inbox. Whoever runs `hudiff wait` on it picks the
// comments up; if nobody is waiting, show the command to run.
// ── Comment authors. "user" is you; anything else is an agent or a named
// reviewer, and gets its own colour so a thread with several is readable.
function who(a){ return !a || a==='user' ? 'me' : a; }
function initial(a){ return who(a)==='me' ? 'me' : a.slice(0,2); }
function authorColor(a){
  if(!a || a==='user') return 'var(--accent)';
  let h=0; for(const ch of a) h=(h*31+ch.charCodeAt(0))%360;
  return `hsl(${h} 45% 45%)`;
}

const AG = {waiting:0, list:[], bound:null, alive:false, at:0};
// One list, two callers. Throttled to the /api/state cadence: no second timer.
async function loadAgents(force){
  if(!force && Date.now()-AG.at < 5000) return;
  AG.at=Date.now();
  try{ const r=await api('GET','/api/agents'); AG.list=r.agents||[]; AG.bound=r.bound; AG.waiting=r.waiting||0; }
  catch(e){ AG.list=[]; }
}
async function refreshWaiting(){
  const st=await api('GET','/api/state'); AG.waiting=st.waiting||0; AG.alive=!!st.bound_alive;
  await loadAgents(); renderSendTarget();
}
const shortPath = p => { const r=S.data.session.repo||''; return p===r ? (r.split('/').pop()||r) : (p.startsWith(r+'/') ? p.slice(r.length+1) : (p.length>40?'…'+p.slice(-39):p)); };
// The list is machine-wide on purpose: the agent you want to review this is
// usually sitting in another worktree. Two groups, this repo first.
function agentRow(a){
  const name = a.name && a.name!==a.kind ? `<span class="k">${esc(a.kind)} ·</span> ${esc(a.name)}` : esc(a.kind);
  return `<div class="ag ${a.same_repo?'':'dim'}" data-pid="${a.pid}" data-name="${esc(a.name)}">`
    + `<span class="st ${a.waiting?'waiting':'idle'}"></span>`
    + `<span class="nm"><span>${name}</span>${a.waiting?'<span class="chip replied">waiting</span>':''}${a.bound?'<span class="chip sent">picked</span>':''}</span>`
    + `<span class="br">${esc(a.branch||'')}</span>`
    + `<span class="cw" title="${esc(a.cwd)}">${esc(shortPath(a.cwd))}</span></div>`;
}
function agentGroups(){
  const same=AG.list.filter(a=>a.same_repo), other=AG.list.filter(a=>!a.same_repo);
  if(!AG.list.length) return '<div class="agempty">No agent running (claude / pi / opencode / codex)</div>';
  return (same.length?`<div class="agsec">This repo</div>${same.map(agentRow).join('')}`:'')
       + (other.length?`<div class="agsec">Other</div>${other.map(agentRow).join('')}`:'');
}
function bindAgentRows(box, onpick){
  box.querySelectorAll('.ag').forEach(r=>r.onclick=e=>{ e.stopPropagation(); onpick(+r.dataset.pid, r.dataset.name); });
}
function renderSendTarget(){
  const el=$('#send-tgt'); if(!el) return;
  const b=AG.bound;
  el.innerHTML = AG.waiting ? `<span class="dot"></span>${AG.waiting} agent waiting`
    : b ? `<span class="dot ${AG.alive?'':'off'}"></span>→ ${esc(b.name||b.kind)}${AG.alive?'':' (gone)'}`
    : '<span class="dot off"></span>no agent waiting';
}
async function toggleAgentPop(force){
  const pop=$('#agentpop'); if(!pop) return;
  const on = force===undefined ? !pop.classList.contains('on') : force;
  if(!on){ pop.classList.remove('on'); return; }
  pop.innerHTML='<div class="agempty">Looking for agents…</div>';
  pop.classList.add('on');
  await loadAgents(true);
  pop.innerHTML=agentGroups();
  bindAgentRows(pop, pid => { pop.classList.remove('on'); doSubmit(pid); });
}
document.addEventListener('click', e => { const pop=$('#agentpop'); if(pop && !e.target.closest('.sendgrp')) pop.classList.remove('on'); });
async function doSubmit(pid){
  const r=await api('POST','/api/submit', pid?{pid}:{});
  AG.waiting=r.waiting||0; if(r.agent){ AG.bound={pid:r.agent.pid,name:r.agent.name,kind:r.agent.kind}; AG.alive=true; }
  renderSendTarget();
  if(r.delivered==='wait') toast(`sent ${r.sent}, woke ${r.waiting} waiting agent(s)`);
  else if(pid) await handOver(pid, `hudiff -s ${S.data.session.id} comments --new`);
  else await offerWaitCommand(r);
  await refreshComments();
}
// Picking an agent always does the same thing and always says what happened:
// the daemon types the command into that agent's terminal, or we copy it.
async function handOver(pid, text){
  const r=await api('POST','/api/deliver',{pid, text});
  const name=(r.agent&&r.agent.name)||'the agent';
  if(r.via==='tmux'||r.via==='kitty'){ toast(`Sent to ${name} via ${r.via}`); return r; }
  if(r.via==='cmux'){ toast(`Sent to ${name} via cmux (unfocused panes may drop it)`); return r; }
  copyText(text);
  await dlg.confirm(`Copied — paste into ${name}`, `${r.detail}.\n\n${text}`, {ok:'Copy again', cancel:'Close'})
    && copyText(text);
  return r;
}
// Nobody picked and nobody is waiting: the comments are pending and the agent
// has to come and get them, so hand over the exact command.
async function offerWaitCommand(r){
  const cmd=r.command||`hudiff -s ${S.data.session.id} wait`;
  const ok=await dlg.confirm(`Marked ${r.sent} comment(s) as pending`,
    `No agent is waiting on this session.\n\nRun this where your agent is:\n\n${cmd}`,
    {ok:'Copy command', cancel:'Close'});
  if(ok) copyText(cmd);
}
async function copyText(t){
  try{ await navigator.clipboard.writeText(t); toast('copied'); }
  catch(e){ toast('copy failed, select it by hand'); }
}
function scrollCodeTo(el, ratio=0.2){
  if(!el) return; const pane=$('#diff'); S.ioLock = Date.now() + 700;
  const go=()=>{ const r=el.getBoundingClientRect(), pr=pane.getBoundingClientRect(); pane.scrollTop += r.top - pr.top - pr.height*ratio; };
  go(); requestAnimationFrame(()=>{ go(); requestAnimationFrame(()=>{ go(); setTimeout(go,120); setTimeout(go,350); }); });
}
function bindTabs(){
  $('#hide-aside').onclick = () => $('#tog-r').click();
  $('#notes').querySelectorAll('.tabs .tab').forEach(b=>b.onclick=()=>{ S.tab=b.dataset.tab; LS.set('tab',S.tab); renderNotes(); });
}
function cmtLoc(c){
  if(c.target==='line') return `${c.side==='new'?'new':'old'} L${c.line}`;
  if(c.target==='hunk') return `whole hunk #${c.hunk}`;
  return 'overall';
}
// authorChips only appear once more than one author has written something.
function authorChips(all){
  const n={}; all.forEach(c=>{ const w=who(c.author); n[w]=(n[w]||0)+1; });
  const names=Object.keys(n);
  if(names.length < 2) return '';
  return `<div class="cbar">${names.map(w=>`<span class="chip au ${C.au&&C.au!==w?'off':''}" data-au="${esc(w)}"><i style="background:${authorColor(w==='me'?'user':w)}"></i>${esc(w)} ${n[w]}</span>`).join('')}</div>`;
}

function renderCommentList(tabs){
  const files=S.data.diff.files; const order=Object.fromEntries(files.map((f,i)=>[f.path,i]));
  const all=S.data.comments.slice().sort((a,b)=>{ const fa=a.file?order[a.file]??999:-1, fb=b.file?order[b.file]??999:-1; if(fa!==fb) return fa-fb; if((a.hunk??-1)!==(b.hunk??-1)) return (a.hunk??-1)-(b.hunk??-1); return (a.line??0)-(b.line??0); });
  const counts={}; all.forEach(c=>counts[c.status]=(counts[c.status]||0)+1);
  const q=C.q.trim().toLowerCase();
  const list=all.filter(c=>!C.au || who(c.author)===C.au).filter(c=>C.st.has(c.status)).filter(c=>!q || (c.body+' '+(c.file||'')+' '+(c.snippet||'')+' '+(c.replies||[]).map(r=>r.body).join(' ')).toLowerCase().includes(q));
  const stcn={open:'unsent',sent:'pending',replied:'replied',resolved:'resolved'};
  let html = tabs + `<div class="cbar"><div class="q"><span class="nf">${U(0xf002)}</span><input id="cq" placeholder="filter comments…" value="${esc(C.q)}"></div></div>
    <div class="cbar">${['open','sent','replied','resolved'].map(st=>`<span class="chip ${st} ${C.st.has(st)?'':'off'}" data-st="${st}">${stcn[st]} ${counts[st]||0}</span>`).join('')}</div>
    ${authorChips(all)}`;
  if(!list.length) html += `<div class="hint" style="text-align:center;padding:30px 0">${all.length?'no matching comments':'No comments yet — click + on a line of code to start'}</div>`;
  list.forEach(c=>{
    const f=c.file||''; const dir=f.includes('/')?f.slice(0,f.lastIndexOf('/')+1):''; const base=f.slice(dir.length);
    let snip=''; if(c.target==='line'){ const [,ln]=find(c); const t=ln?ln.t:(c.snippet!=null?' ':null); const txt=ln?ln.s:c.snippet; if(txt!=null) snip=`<div class="snip ${t==='+'?'add':t==='-'?'del':''}">${t===' '?' ':(t||' ')} ${esc(txt)}</div>`; }
    else if(c.target==='hunk'){ const fi=order[c.file]; const h=files[fi]?.hunks[c.hunk]; if(h) snip=`<div class="snip">${esc(h.header)}</div>`; }
    const last=(c.replies||[]).at(-1);
    html += `<div class="ccard ${c.status}" data-id="${c.id}"><div class="ch">${f?`<span class="f" title="${esc(f)}"><span class="d">${esc(dir)}</span>${esc(base)}</span>`:`<span class="f">overall</span>`}<span class="ln">${cmtLoc(c)}</span><span class="chip ${c.status}">${stcn[c.status]}</span>${c.stale?'<span class="chip stale">⚠</span>':''}</div>${snip}<div class="cb">${inline(c.body)}</div>
      <div class="cf"><span class="last">${last?`<b>${last.who==='agent'?'agent':'me'}:</b> ${esc(last.body.split('\n')[0])}`:'no replies'}</span><span>${(c.replies||[]).length?(c.replies.length+' replies'):''}</span><span class="hint">${c.id}</span></div></div>`;
  });
  $('#notes').innerHTML = html;
  bindTabs();
  $('#cq').oninput = e => { C.q=e.target.value; const pos=e.target.selectionStart; renderCommentList(tabs); const i=$('#cq'); i.focus(); i.setSelectionRange(pos,pos); };
  $('#cq').onkeydown = e => e.stopPropagation();
  $('#notes').querySelectorAll('.cbar .chip[data-au]').forEach(ch=>ch.onclick=()=>{ C.au = C.au===ch.dataset.au ? null : ch.dataset.au; renderCommentList(tabs); });
  $('#notes').querySelectorAll('.cbar .chip[data-st]').forEach(ch=>ch.onclick=()=>{ const st=ch.dataset.st; if(C.st.has(st)) C.st.delete(st); else C.st.add(st); renderCommentList(tabs); });
  $('#notes').querySelectorAll('.ccard').forEach(el=>el.onclick=()=>jumpToComment(el.dataset.id));
}
function find(c){ const fi=S.data.diff.files.findIndex(f=>f.path===c.file); const h=S.data.diff.files[fi]?.hunks[c.hunk]; if(!h) return [null,null]; const ln=h.lines.find(l=>(c.side==='new'?l.n:l.o)===c.line && !(c.side==='new'&&l.t==='-') && !(c.side==='old'&&l.t==='+')); return [h,ln||null]; }
function jumpToComment(id){
  const c=S.data.comments.find(x=>x.id===id); if(!c) return;
  if(c.status==='resolved' && !S.showResolved){ S.showResolved=true; $('#showres').checked=true; renderComments(); }
  const flash=()=>{ const th=document.querySelector(`.thread[data-id="${id}"]`); if(th){ th.classList.remove('flash'); void th.offsetWidth; th.classList.add('flash'); th.scrollIntoView({block:'center'}); } };
  if(c.target==='general'){ S.tab='notes'; LS.set('tab','notes'); renderNotes(); flash(); return; }
  const fi=S.data.diff.files.findIndex(f=>f.path===c.file); if(fi<0){ toast('the file this comment belongs to is not in this diff'); return; }
  showMid('code');
  const fb=document.getElementById('f-'+fi); fb.classList.remove('collapsed');
  let anchor = c.target==='line' ? document.querySelector(`tr.line[data-fi="${fi}"][data-hi="${c.hunk}"][data-side="${c.side}"][data-line="${c.line}"]`) : document.querySelector(`tr.hh[data-fi="${fi}"][data-hi="${c.hunk}"]`);
  if(!anchor){ anchor=document.getElementById(`h-${fi}-${c.hunk}`); }
  scrollCodeTo(anchor, 0.2);
  setActive(fi, c.hunk);
  flash();
}
function highlightLines(fi, hi, spec, on){
  const t=document.getElementById(`h-${fi}-${hi}`); if(!t) return;
  let side='new', a, b;
  const m = /^(?:(old|new):)?L?(\d+)(?:\s*[-–]\s*L?(\d+))?/i.exec(spec||'');
  if(m){ side=(m[1]||'new').toLowerCase(); a=+m[2]; b=+(m[3]||m[2]); }
  t.querySelectorAll('tr.line').forEach(tr=>{
    const ok = m ? (tr.dataset.side===side && +tr.dataset.line>=a && +tr.dataset.line<=b) : tr.classList.contains('add')||tr.classList.contains('del');
    tr.classList.toggle('hl', on && ok);
  });
}

// ── Keymap (spec §6). The daemon merges config.keymap over the defaults and
// serves the table at /api/config; single keys are ignored while typing.
let KEYS = {};
async function loadKeymap(){
  try{ const r=await fetch('/api/config'); KEYS=(await r.json()).keymap||{}; }catch(e){ KEYS={}; }
}
const ACTIONS = {
  next_hunk: () => stepHunk(1), prev_hunk: () => stepHunk(-1),
  next_file: () => stepFile(1), prev_file: () => stepFile(-1),
  next_comment: () => stepComment(1), prev_comment: () => stepComment(-1),
  tab_code: () => showMid('code'), tab_diagrams: () => showMid('diagrams'), tab_review: () => showMid('review'),
  comment: () => clickFirst(`table.hunk[data-fi="${S.active}"][data-hi="${S.activeHunk}"] tr.line .gut, table.hunk tr.line .gut`),
  comment_overall: () => clickFirst('#gen-add'),
  send: () => clickFirst('#submit'),
  toggle_resolved: () => clickFirst('#showres'),
  filter_files: () => { if($('#main').classList.contains('nav-off')) $('#tog-l').click(); $('#q').focus(); },
  toggle_nav: () => clickFirst('#tog-l'), toggle_aside: () => clickFirst('#tog-r'),
  theme: () => clickFirst('#theme'),
  help: showKeymap,
  escape: () => { if($('#lb').style.display!=='none') return closeLightbox(); if(S.pin){ S.pin=null; applyPin(); } },
};
function clickFirst(sel){ const el=document.querySelector(sel); if(el) el.click(); }
function hunkTables(){ return [...document.querySelectorAll('table.hunk')].filter(t=>!t.closest('.collapsed')); }
function stepHunk(d){
  const all=hunkTables(); if(!all.length) return;
  let i=all.findIndex(t=>+t.dataset.fi===S.active&&+t.dataset.hi===S.activeHunk);
  i=Math.max(0, Math.min(i+d, all.length-1));
  showMid('code'); scrollCodeTo(all[i], 0.05); setActive(+all[i].dataset.fi, +all[i].dataset.hi);
}
function stepFile(d){
  const files=[...document.querySelectorAll('.file-block')]; if(!files.length) return;
  let i=files.findIndex(f=>+f.dataset.fi===S.active);
  i=Math.max(0, Math.min((i<0?0:i)+d, files.length-1));
  showMid('code'); scrollCodeTo(files[i], 0.02); setActive(+files[i].dataset.fi, null);
}
function stepComment(d){
  const th=[...document.querySelectorAll('#diff .thread')]; if(!th.length) return;
  S.cIdx = Math.max(0, Math.min((S.cIdx ?? -1) + d, th.length-1));
  showMid('code'); scrollCodeTo(th[S.cIdx], 0.2);
}
function showKeymap(){
  const rows=Object.entries(KEYS).map(([a,k])=>`${a.padEnd(18)}${k}`).join('\n');
  dlg.confirm('Keyboard shortcuts', rows || 'no keymap loaded', {ok:'Close', cancel:'Close'});
}
document.addEventListener('keydown', e => {
  if(e.target.tagName==='TEXTAREA'||e.target.tagName==='INPUT') return;
  if(e.metaKey||e.ctrlKey||e.altKey) return;
  if(e.key==='/'){ e.preventDefault(); ACTIONS.filter_files(); return; }
  if(e.key==='v'){ showMid({code:'diagrams',diagrams:'review',review:'code'}[S.mid]); return; }
  if(e.key==='0' && $('#lb').style.display!=='none'){ $('#lb-img').onload(); return; }
  for(const [action, key] of Object.entries(KEYS)){
    if(key===e.key && ACTIONS[action]){ e.preventDefault(); ACTIONS[action](); return; }
  }
});

// Poll: the agent wrote notes, or comments changed elsewhere → refresh just that part
setInterval(async () => {
  if(document.hidden) return;
  try{
    const r = await fetch(BASE+'/api/state'); if(!r.ok) throw 0; const st = await r.json();
    if($('#offline').style.display!=='none'){ $('#offline').style.display='none'; toast('reconnected'); await loadAll(); return; }
    if(!S.state) return;
    if(st.diff_v!==S.state.diff_v){ await loadAll(); toast('diff refreshed'); return; }
    if(st.notes_v!==S.state.notes_v){ const d=await api('GET','/api/session'); S.data.notes=d.notes; S.data.diagrams=d.diagrams; S.state=d.state; renderHeader(); renderDiff(); renderComments(); renderMidbar(); renderDiagrams(); markReviewLines(); renderNotes(); toast('notes updated'); return; }
    if(st.waiting!==AG.waiting || !!st.bound_alive!==AG.alive){ AG.waiting=st.waiting||0; AG.alive=!!st.bound_alive; renderSendTarget(); if($('#agentpop')&&$('#agentpop').classList.contains('on')) await toggleAgentPop(true); }
    if(st.review_v!==S.state.review_v){ await refreshReview(); toast('review report updated'); return; }
    if(st.comments_v!==S.state.comments_v){ await refreshComments(); }
  }catch(e){ $('#offline').style.display='block'; }
}, 5000);

loadAll().then(()=>{ const m=/^#f-(\d+)$/.exec(location.hash); if(m){ const el=document.getElementById('f-'+m[1]); if(el){ el.scrollIntoView({block:'start'}); setActive(+m[1]); } } });
