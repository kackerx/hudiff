'use strict';
const S = {data:null, state:null, active:0, activeHunk:null, showResolved:false, openForms:new Set(), theme:'auto', nerd:true, sumOpen:false, pin:null, tab:'notes', mid:'code', activeDg:null};
const C = { q:'', st:new Set(['open','sent','replied']) };   // 评论列表: 过滤词 / 显示哪些状态
const LS = { get(k,d){ try{ const v=localStorage.getItem('hudiff.'+k); return v===null?d:JSON.parse(v); }catch(e){ return d; } }, set(k,v){ try{ localStorage.setItem('hudiff.'+k, JSON.stringify(v)); }catch(e){} } };

// ── Nerd Font 图标(码点全部用转义写, 别直接贴字形) ──
// 文件类型: [字形, 颜色]。参考 nvim-web-devicons 的配色。
// 码点用 U(0x....) 写, 【不要】直接贴字形 —— 私有区字符在编辑链路里会被静默丢掉(已经发生过一次)。
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
// git 状态: 修改 / 新增 / 删除 / 重命名 (FontAwesome 的方框系, 和 yazi 的 git 插件一路)
const GS = { M:[U(0xf044),'修改'], A:[U(0xf196),'新增'], D:[U(0xf147),'删除'], R:[U(0xf074),'重命名'] };
function iconFor(path){
  const base=path.split('/').pop(); const lower=base.toLowerCase();
  if(/^dockerfile/.test(lower)) return FT.dockerfile; if(/^makefile$/.test(lower)) return FT.makefile;
  if(/^\.tmux\.conf|^kitty\.conf$/.test(lower)) return FT.conf;
  if(/^\.?(bash|zsh)rc$|^\.?(bash|zsh)_profile$|^\.zshenv$/.test(lower)) return FT.sh;
  if(lower==='claude.md'||lower==='agents.md') return FT.md;
  const ext = lower.includes('.') ? lower.split('.').pop() : lower;
  if(FT[ext]) return FT[ext];
  if(!lower.includes('.')){ return ['','#89e051']; }   // 无扩展名多半是脚本
  return FT_DEFAULT;
}
const icHTML = (path) => { if(!S.nerd) return ''; const [ch,color]=iconFor(path); return `<span class="ic nf" style="color:${color}">${ch}</span>`; };
const gsHTML = (st) => S.nerd ? `<span class="gs nf ${st}" title="${GS[st]?.[1]||st}">${GS[st]?.[0]||st}</span>` : `<span class="st ${st}">${st}</span>`;
const BASE = location.pathname.replace(/\/+$/,'');   // /s/<id>
const $ = (sel, el=document) => el.querySelector(sel);
const esc = s => String(s??'').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const toast = m => { const t=$('#toast'); t.textContent=m; t.classList.add('on'); clearTimeout(t._t); t._t=setTimeout(()=>t.classList.remove('on'),2000); };

// ── 主题: auto / light / dark, 记在 localStorage ──
function applyTheme(t){
  S.theme=t; try{ localStorage.setItem('hudiff.theme', t); }catch(e){}
  if(t==='auto') delete document.documentElement.dataset.theme; else document.documentElement.dataset.theme=t;
  const dark = t==='dark' || (t==='auto' && matchMedia('(prefers-color-scheme: dark)').matches);
  $('#hljs-light').disabled = dark; $('#hljs-dark').disabled = !dark;
  const b=$('#theme'); if(b) b.textContent = {auto:'◐ 自动', light:'☀ 浅色', dark:'☾ 深色'}[t];
  if(S.data){ renderDiagrams(); if(S.mid==='diagrams') renderNotes(); }
}
try{ S.theme = localStorage.getItem('hudiff.theme') || 'auto'; }catch(e){}
S.tab = LS.get('tab', 'notes');
S.mid = LS.get('mid', 'code'); { const v=new URLSearchParams(location.search).get('view'); if(v==='code'||v==='diagrams'||v==='review') S.mid=v; } { const t=new URLSearchParams(location.search).get('tab'); if(t==='notes'||t==='comments') S.tab=t; }   // ?tab=comments 预设
S.sumOpen = LS.get('sum.'+location.pathname.split('/')[2], false);

// ── 侧栏: 拖宽 / 折叠, 记在 localStorage ──
function setupSash(){
  const main=$('#main');
  const W={nav:LS.get('w.nav',250), aside:LS.get('w.aside',400)};
  const OFF={nav:LS.get('off.nav',false), aside:LS.get('off.aside',false)};
  const qs=new URLSearchParams(location.search); if(qs.has('nav')) OFF.nav = qs.get('nav')==='0'; if(qs.has('aside')) OFF.aside = qs.get('aside')==='0';   // ?nav=0&aside=0 预设折叠(不持久化)
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
  if(qs.has('perf')){ const im=new Image(); im.src=BASE+'/api/hang?ms=14000'; document.body.appendChild(im); }
  if(qs.has('perf')) setTimeout(async()=>{ try{
    const P={}; const wrap=n=>{ const f=window[n]; if(typeof f!=='function') return; window[n]=function(...a){ const t=performance.now(); const r=f.apply(this,a); P[n]=P[n]||{n:0,ms:0}; P[n].n++; P[n].ms+=performance.now()-t; return r; }; };
    ['renderTree','renderDiff','renderComments','renderNotes','renderHeader','renderDiagrams','applyPin','markNoted','setActive'].forEach(wrap);
    const t0=performance.now(); renderDiff(); const tDiff=performance.now()-t0;
    const t1=performance.now(); renderTree(); const tTree=performance.now()-t1;
    const t2=performance.now(); renderNotes(); const tNotes=performance.now()-t2;
    const t3=performance.now(); renderComments(); const tCm=performance.now()-t3;
    const el=$('#diff'); const t4=performance.now(); for(let i=0;i<40;i++){ el.scrollTop=i*400; await new Promise(r=>setTimeout(r,16)); } const tScroll=performance.now()-t4;
    const t5=performance.now(); for(let i=0;i<20;i++){ document.documentElement.classList.toggle('x'); await new Promise(r=>setTimeout(r,16)); } const tCls=performance.now()-t5;
    document.title = `nodes=${document.getElementsByTagName('*').length} renderDiff=${tDiff.toFixed(0)} tree=${tTree.toFixed(0)} notes=${tNotes.toFixed(0)} comments=${tCm.toFixed(0)} scroll40=${tScroll.toFixed(0)} rootcls20=${tCls.toFixed(0)} | `+Object.entries(P).map(([k,v])=>`${k}:${v.n}x${(v.ms/v.n).toFixed(1)}`).join(' ');
  }catch(e){ document.title='PERF ERR '+e.message; } }, 1500);
  if(qs.has('debug')) setTimeout(()=>{ document.title = 'IMGS=' + [...document.querySelectorAll('.dg img')].map(i=>i.closest('.dg').dataset.id+':'+i.complete+':'+i.naturalWidth+'x'+i.naturalHeight+':'+i.getBoundingClientRect().width+'x'+i.getBoundingClientRect().height).join(' | ') + ' COLS='+getComputedStyle(main).gridTemplateColumns+' | nav='+getComputedStyle($('#tree')).display+' | sashl='+$('#sash-l').getBoundingClientRect().width; }, 500);
}
setupSash();
matchMedia('(prefers-color-scheme: dark)').addEventListener('change', ()=>applyTheme(S.theme));

// ── 语法高亮: 按扩展名选语言, 逐行喂 hljs; 没加载到 hljs 就退回纯转义 ──
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

// 极简 markdown: 只做 agent 说明里会用到的那几样, 全部先转义再渲染
function md(src){
  if(!src) return '';
  src = String(src).replace(/\\n/g, '\n');   // agent 偶尔把换行写成字面量 \n, 宽容处理
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
  if(!r.ok){ toast('出错: '+(await r.text())); throw new Error(path); }
  return r.json();
}

let treeReady=false;
async function loadAll(){
  S.data = await api('GET','/api/session'); S.state = S.data.state;
  if(!treeReady){ treeReady=true; T.closed=new Set(LS.get('closed.'+S.data.session.id, [])); setupTree(); }
  renderHeader(); renderTree(); renderDiff(); renderComments(); renderMidbar(); renderDiagrams(); renderReview(); markReviewLines(); renderNotes();
  loadAgents().catch(()=>{});
  if(new URLSearchParams(location.search).has('agents')) setTimeout(()=>toggleAgentPop(true), 300);   // ?agents 预设: 打开选择弹层(截图/调试用)
}

const kindCN = {worktree:'工作区', commit:'commit', range:'区间', pr:'PR', patch:'补丁'};
function renderHeader(){
  const {session:s, diff, notes} = S.data;
  const adds = diff.files.reduce((a,f)=>a+f.add,0), dels = diff.files.reduce((a,f)=>a+f.del,0), hunks = diff.files.reduce((a,f)=>a+f.hunks.length,0);
  const m = s.meta||{};
  const parts = [`<span>${diff.files.length} 个文件 · ${hunks} 个 hunk</span>`, `<span><b class="add">+${adds}</b> <b class="del">−${dels}</b></span>`];
  if(m.rev) parts.push(`<span><code>${esc(m.rev)}</code>${m.base?` ← <code>${esc(m.base)}</code>`:''}</span>`);
  else if(m.base) parts.push(`<span>对比基线 <code>${esc(m.base)}</code></span>`);
  if(m.author) parts.push(`<span>${esc(m.author)}</span>`); if(m.date) parts.push(`<span>${esc(m.date)}</span>`);
  if(m.branch) parts.push(`<span>分支 <code>${esc(m.branch)}</code></span>`);
  if(m.url) parts.push(`<a href="${esc(m.url)}" target="_blank">${esc(m.url.replace(/^https?:\/\//,''))}</a>`);
  parts.push(`<span class="hint">${esc(s.created?.slice(0,16).replace('T',' ')||'')}</span>`);
  parts.push(`<span class="hint" title="hudiff -s ${esc(s.id)} …">会话 <code>${esc(s.id)}</code></span>`);
  if(s.archived) parts.push(`<span class="chip" style="background:var(--bg3)">已归档 ${esc(String(s.archived).slice(0,10))}</span>`);
  const title = notes.title || s.title;
  const unsent = S.data.comments.filter(c=>c.status==='open').length;
  $('#hdr').innerHTML = `
    <div class="row">
      <a href="/" class="hint" title="全部会话" style="white-space:nowrap">☰ 全部会话</a>
      <h1><a class="brand" href="/" title="全部会话"><svg class="mark" viewBox="0 0 64 64" width="20" height="20" fill="none" stroke-width="6.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21V9h12M43 9h12v12M9 43v12h12M55 43v12H43" style="stroke:var(--fg3)"/><path d="M22 26h15" style="stroke:var(--del-mark)"/><path d="M22 38h20" style="stroke:var(--add-mark)"/></svg></a><span class="kind ${esc(s.kind)}">${esc(kindCN[s.kind]||s.kind)}</span>${esc(title)}</h1>
      <div class="toolbar">
        <button class="ghost sm" id="archive" title="${s.archived?'取消归档, 重新出现在首页':'归档: 首页不再列出, 数据保留, 链接仍可用'}">${s.archived?'↩ 取消归档':'⌸ 归档'}</button>
        <label class="hint" style="display:flex;gap:4px;align-items:center"><input type="checkbox" id="showres" ${S.showResolved?'checked':''}> 显示已解决</label>
        <button class="ghost sm" id="theme"></button>
        ${unsent?`<span class="cnt">${unsent} 条未发送</span>`:''}
        <span class="sendgrp"><button class="primary" id="submit" ${unsent?'':'disabled'} title="有 agent 在 wait 就直接叫醒; 否则通知绑定的 agent">发给 agent</button><button class="drop" id="agents-btn" title="选择 / 更换接收的 agent">▾</button><span class="tgt" id="send-tgt"></span><div class="agentpop" id="agentpop"></div></span>
      </div>
    </div>
    <div class="meta">${parts.join('')}</div>
    ${s.brief&&(s.brief.prompt||s.brief.presets?.length)?`<div class="brief"><span class="k">审查要求</span><span class="v">${esc([s.brief.prompt, ...(s.brief.presets||[]).map(p=>p.text)].filter(Boolean).join('\n'))}</span></div>`:''}
    <div class="summary ${notes.summary?(S.sumOpen?'':'collapsed'):'empty'}" id="summary">${notes.summary?`<div class="md">${md(notes.summary)}</div><span class="tog">${S.sumOpen?'收起 ▴':'展开 ▾'}</span>`:'agent 还没写整体说明, 页面会自动刷新…'}</div>
    ${m.body?`<details class="body"><summary>原始 ${s.kind==='pr'?'PR 描述':'提交说明'}</summary><pre>${esc(m.body)}</pre></details>`:''}`;
  $('#showres').onchange = e => { S.showResolved = e.target.checked; renderComments(); renderNotes(); };
  $('#submit').onclick = () => doSubmit(null);
  $('#archive').onclick = async () => { const to=!s.archived; if(to && !(await dlg.confirm('归档这个会话?', '首页不再列出, 数据和链接都保留, 随时可以取消归档。', {ok:'归档'}))) return; await fetch('/api/s/'+encodeURIComponent(s.id),{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({archived:to})}); const d=await api('GET','/api/session'); S.data.session=d.session; renderHeader(); toast(to?'已归档':'已取消归档'); };
  $('#agents-btn').onclick = e => { e.stopPropagation(); toggleAgentPop(); };
  renderSendTarget();
  $('#theme').onclick = () => applyTheme({auto:'light',light:'dark',dark:'auto'}[S.theme]);
  if(notes.summary) $('#summary').onclick = e => { if(e.target.closest('a')) return; S.sumOpen=!S.sumOpen; LS.set('sum.'+s.id, S.sumOpen); const el=$('#summary'); el.classList.toggle('collapsed', !S.sumOpen); el.querySelector('.tog').textContent = S.sumOpen?'收起 ▴':'展开 ▾'; };
  applyTheme(S.theme);
}

const T = { q:'', exts:null, closed:new Set() };   // 目录树: 过滤词 / 勾选的扩展名(null=全部) / 折叠的目录
function extOf(path){ const b=path.split('/').pop(); return b.includes('.') ? '.'+b.split('.').pop().toLowerCase() : '(无扩展名)'; }
function renderTree(){
  const files=S.data.diff.files;
  const q=T.q.trim().toLowerCase();
  const visible = new Set(files.map((f,i)=>i).filter(i=>{ const f=files[i]; if(q && !f.path.toLowerCase().includes(q)) return false; if(T.exts && !T.exts.has(extOf(f.path))) return false; return true; }));
  const root = {dirs:{}, files:[]};
  files.forEach((f,i)=>{ if(!visible.has(i)) return; const parts=f.path.split('/'); let n=root; parts.slice(0,-1).forEach(p=>{ n=n.dirs[p]??=({dirs:{},files:[]}); }); n.files.push({f,i,name:parts.at(-1)}); });
  const squash=(n,name)=>{ while(Object.keys(n.dirs).length===1 && n.files.length===0){ const k=Object.keys(n.dirs)[0]; name=name+'/'+k; n=n.dirs[k]; } return [n,name]; };
  // 评论标记: 有未解决的 → 实心气泡 + 数量(未发送用黄, 其它用蓝); 全解决 → 打勾气泡, 淡绿
  const cmark = fi => { const cs=S.data.comments.filter(c=>c.file===files[fi].path); if(!cs.length) return '';
    const live=cs.filter(c=>c.status!=='resolved'), open=live.filter(c=>c.status==='open').length;
    const tip=`${cs.length} 条评论: ${open} 未发送 / ${live.length-open} 已发或已回复 / ${cs.length-live.length} 已解决`;
    if(live.length) return S.nerd ? `<span class="cm nf ${open?'open':'live'}" title="${tip}">${U(0xf075)}</span>` : `<span class="cc ${open?'open':'live'}" title="${tip}">${live.length}</span>`;
    return S.nerd ? `<span class="cm nf done" title="${tip}">${U(0xf0e5)}</span>` : `<span class="cc done" title="${tip}">✓</span>`; };
  let html='';
  const walk=(n,d,prefix,hidden)=>{
    n.files.sort((a,b)=>a.name.localeCompare(b.name)).forEach(({f,i,name})=>{
      html+=`<div class="file ${i===S.active?'active':''}" data-fi="${i}" style="--d:${d};${hidden?'display:none':''}" title="${esc(f.path)}">${icHTML(f.path)}<span class="name">${esc(name)}</span>${cmark(i)}<span class="pm"><span class="a">+${f.add}</span> <span class="d">−${f.del}</span></span>${gsHTML(f.status)}</div>`; });
    Object.keys(n.dirs).sort().forEach(k=>{ const [sub,name]=squash(n.dirs[k],k); const full=prefix?prefix+'/'+name:name; const closed=!q && T.closed.has(full);
      html+=`<div class="dir ${closed?'closed':''}" data-path="${esc(full)}" style="--d:${d};${hidden?'display:none':''}"><span class="chev">▾</span>${S.nerd?`<span class="ic nf">${FOLDER}</span>`:''}${esc(name)}</div>`; walk(sub,d+1,full,hidden||closed); });
  };
  walk(root,0,'',false);
  if(!visible.size) html=`<div class="empty">没有匹配的文件</div>`;
  $('#tree-list').innerHTML = html;
  $('#tree-list').querySelectorAll('.file').forEach(el=>el.onclick=()=>{ showMid('code'); const fb=document.getElementById('f-'+el.dataset.fi); fb.classList.remove('collapsed'); scrollCodeTo(fb, 0); setActive(+el.dataset.fi); });
  $('#tree-list').querySelectorAll('.dir').forEach(el=>el.onclick=()=>{ const p=el.dataset.path; if(T.closed.has(p)) T.closed.delete(p); else T.closed.add(p); LS.set('closed.'+S.data.session.id, [...T.closed]); renderTree(); });
  // 扩展名下拉
  const counts={}; files.forEach(f=>{ const e=extOf(f.path); counts[e]=(counts[e]||0)+1; });
  const exts=Object.keys(counts).sort();
  $('#extf-list').innerHTML = `<label class="all"><input type="checkbox" id="ext-all" ${T.exts?'':'checked'}> 全部 <span class="n">${files.length}</span></label>` + exts.map(e=>`<label><input type="checkbox" data-ext="${esc(e)}" ${!T.exts||T.exts.has(e)?'checked':''}> ${esc(e)} <span class="n">${counts[e]}</span></label>`).join('');
  $('#extf summary').classList.toggle('on', !!T.exts);
  $('#extf summary').textContent = T.exts ? `类型 ${T.exts.size}/${exts.length}` : '类型';
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
    const stcn = {A:'新增',D:'删除',R:'重命名',M:'修改'}[f.status]||f.status;
    html+=`<div class="file-block" id="f-${fi}" data-fi="${fi}"><div class="file-head" data-fi="${fi}"><span class="tri">▾</span><span class="st ${f.status}">${stcn}</span><span class="path"><span class="dir">${esc(dir)}</span>${esc(base)}${f.status==='R'?` <span class="hint">← ${esc(f.old_path)}</span>`:''}</span>${fn.summary?`<span class="fsum">· ${inline(fn.summary)}</span>`:'<span class="fsum"></span>'}<span class="pm"><b style="color:var(--add-fg)">+${f.add}</b> <b style="color:var(--del-fg)">−${f.del}</b></span></div>`;
    if(f.binary) html+=`<div class="hint binhint">二进制文件</div>`;
    f.hunks.forEach((h,hi)=>{
      html+=`<table class="hunk" id="h-${fi}-${hi}" data-fi="${fi}" data-hi="${hi}"><colgroup><col style="width:48px"><col style="width:48px"><col style="width:24px"><col></colgroup><tr class="hh" data-fi="${fi}" data-hi="${hi}"><td colspan="4"><span class="hno ${(()=>{const x=(notes[f.path]?.hunks||{})[String(hi)]; return x&&!x.skip?'noted':'';})()}" title="跳到这段的说明">#${hi}</span>${esc(h.header)}${(()=>{const x=(notes[f.path]?.hunks||{}); return (String(hi) in x) && (x[String(hi)]===null||x[String(hi)]?.skip) ? '<span class="skip">· 无需说明</span>' : '';})()}<button class="sm hbtn" data-act="hunkc" data-fi="${fi}" data-hi="${hi}" title="评论整段">💬 整段</button></td></tr>`;
      h.lines.forEach(l=>{
        const cls = l.t==='+'?'add':l.t==='-'?'del':'ctx';
        const side = l.t==='-'?'old':'new'; const ln = side==='new'?l.n:l.o;
        html+=`<tr class="line ${cls}" data-fi="${fi}" data-hi="${hi}" data-side="${side}" data-line="${ln}"><td class="num">${l.o??''}</td><td class="num">${l.n??''}</td><td class="gut" title="在这一行评论"></td><td class="code"><span class="m">${l.t===' '?'':l.t}</span>${hl(l.s, lang)}${l.nonl?'<span class="hint" title="文件末尾没有换行"> ⏎∅</span>':''}</td></tr>`;
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
    // 反向: 点代码行 / hunk 头 → 跳到对应说明卡片(正在选文字时不动)
    const tr = e.target.closest('tr[data-fi]'); if(!tr) return;
    if(e.target.closest('.crow')) return;
    const fi=+tr.dataset.fi, hi=+tr.dataset.hi;
    if(e.target.closest('.hno')){ focusCard(fi, hi); return; }          // #N 标记: 总是跳
    if(!getSelection().isCollapsed) return;
    if(!tr.classList.contains('line')) return;
    // 代码行: 只有说明【覆盖到】这一行才跳; 同 hunk 但在 lines 范围外的行不算
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

// ── 评论 ──
function threadHTML(c){
  const stale = c.stale?`<span class="chip stale">⚠ 行已变</span>`:'';
  const stcn = {open:'未发送',sent:'已发给 agent',replied:'agent 已回复',resolved:'已解决'}[c.status]||c.status;
  const adone = c.agent_done && c.status!=='resolved' ? `<span class="chip" style="background:var(--ok-bg);color:var(--ok)">✔ agent 已处理, 待你确认</span>` : '';
  const rootUnsent = c.status==='open' && !(c.replies||[]).some(r=>r.who==='user');
  const rootChip = c.status==='open' ? (rootUnsent?`<span class="chip open">未发送</span>`:`<span class="chip sent">已发送</span>`) : `<span class="chip ${c.status}">${stcn}</span>`;
  let h = `<div class="thread" data-id="${c.id}"><div class="meta"><span class="av">我</span><span class="who">我</span>${rootChip}${adone}${stale}<span>${esc(c.created?.slice(5,16).replace('T',' ')||'')}</span><span class="hint">${c.id}</span></div><div class="msg">${inline(c.body)}</div>`;
  (c.replies||[]).forEach(r=>{ const ag=r.who==='agent'; const rc = ag ? '' : (r.sent===false ? `<span class="chip open">未发送</span>` : (r.sent===true ? `<span class="chip sent">已发送</span>` : '')); h+=`<div class="reply"><div class="meta"><span class="av ${ag?'agent':''}">${ag?'A':'我'}</span><span class="who ${r.who}">${ag?'agent':'我'}</span>${rc}<span>${esc(r.ts?.slice(5,16).replace('T',' ')||'')}</span></div><div class="msg">${ag?md(r.body):inline(r.body)}</div></div>`; });
  h += `<div class="acts"><button class="sm" data-act="reply">回复</button>${c.status==='resolved'?'<button class="sm" data-act="reopen">重开</button>':`<button class="sm ${c.agent_done?'primary':''}" data-act="resolve">${c.agent_done?'确认解决':'解决'}</button>`}${(c.replies||[]).length?'':'<button class="sm" data-act="edit">编辑</button>'}<button class="sm ghost" data-act="del">删除</button></div></div>`;
  return h;
}
function bindThreadActs(container){
  container.querySelectorAll('.thread .acts button').forEach(b=>b.onclick=async(e)=>{
    e.stopPropagation();
    const th=b.closest('.thread'), id=th.dataset.id, c=S.data.comments.find(x=>x.id===id);
    const act=b.dataset.act;
    if(act==='del'){ if(!(await dlg.confirm('删除这条评论?', '连同下面的回复一起删, 不可恢复。', {ok:'删除', danger:true}))) return; await api('DELETE','/api/comments/'+id); return refreshComments(); }
    if(act==='resolve'){ await api('PATCH','/api/comments/'+id,{status:'resolved'}); return refreshComments(); }
    if(act==='reopen'){ await api('PATCH','/api/comments/'+id,{status:'open'}); return refreshComments(); }
    if(act==='reply'){ if(th.querySelector('.cform')) return; const f=formEl('', async v=>{ await api('POST',`/api/comments/${id}/reply`,{who:'user',body:v}); await refreshComments(); }, ()=>f.remove()); th.appendChild(f); f.querySelector('textarea').focus(); return; }
    if(act==='edit'){ if(th.querySelector('.cform')) return; const f=formEl(c.body, async v=>{ await api('PATCH','/api/comments/'+id,{body:v}); await refreshComments(); }, ()=>f.remove()); th.appendChild(f); f.querySelector('textarea').focus(); return; }
  });
}
function formEl(initial, onSave, onCancel){
  const d=document.createElement('div'); d.className='cform';
  d.innerHTML=`<textarea placeholder="写意见… (⌘/Ctrl+Enter 保存, Esc 取消)">${esc(initial)}</textarea><div class="acts"><button class="primary sm" data-a="save">保存</button><button class="sm" data-a="cancel">取消</button><span>保存后还要点右上角「发给 agent」才会通知 agent</span></div>`;
  const ta=d.querySelector('textarea');
  const save=async()=>{ const v=ta.value.trim(); if(!v) return toast('空的'); await onSave(v); };
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
    S.openForms.delete(key); tr.remove(); await refreshComments(); toast('已保存');
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
  if(unsent){ if(!cnt){ const s=document.createElement('span'); s.className='cnt'; sub.before(s); } $('#hdr .cnt').textContent=`${unsent} 条未发送`; } else if(cnt) cnt.remove();
  sub.disabled=!unsent;
  renderTree(); renderMidbar();
}
async function refreshComments(){ const d = await api('GET','/api/session'); S.data.comments=d.comments; S.state=d.state; renderComments(); renderNotes(); }

// ── 右侧说明 ──
function renderNotes(){
  const f = S.data.diff.files[S.active]; if(!f){ $('#notes').innerHTML=''; return; }
  const fn = (S.data.notes.files||{})[f.path]||{};
  const gen = S.data.comments.filter(c=>c.target==='general' && (S.showResolved||c.status!=='resolved'));
  const live = S.data.comments.filter(c=>c.status!=='resolved').length;
  const tabs = `<div class="tabs"><button class="tab ${S.tab==='notes'?'on':''}" data-tab="notes">解读</button><button class="tab ${S.tab==='comments'?'on':''}" data-tab="comments">评论<span class="n">${live}</span></button><span class="sp"></span><button class="ghost sm" id="hide-aside" title="收起右栏  ]">◨</button></div>`;
  if(S.tab==='comments'){ renderCommentList(tabs); return; }
  if(S.mid==='diagrams'){ renderDiagramList(tabs.replace('>解读<', '>图列表<')); return; }
  if(S.mid==='review'){ renderReviewOutline(tabs.replace('>解读<', '>审查项<')); return; }
  let html = tabs + `<div class="gen"><h3>整体意见</h3>${gen.map(threadHTML).join('')}<button class="sm" id="gen-add">+ 对整个改动的意见</button></div>`;
  html += `<h3>代码解读 · <span class="fname">${esc(f.path.split('/').pop())}</span></h3>`;
  if(fn.summary) html+=`<div class="fsum-box">${inline(fn.summary)}</div>`;
  const hunks = fn.hunks||{};
  const realCards = Object.values(hunks).filter(x=>x && !x.skip).length;
  if(!Object.keys(hunks).length) html+=`<div class="card pending">agent 还没写这个文件的说明…</div>`;
  else if(!realCards) html+=`<div class="card pending">这个文件的改动都是机械性的, agent 没有单独说明</div>`;
  f.hunks.forEach((h,hi)=>{
    const n = hunks[String(hi)]; if(!n || n.skip) return;
    const isOld=/^old/i.test(n.lines||'');
    const rng = n.lines ? `<span class="chip rng ${isOld?'old':''}">${isOld?'旧版':'新版'} L${esc(String(n.lines).replace(/^(old|new):/i,''))}</span>` : `<span class="chip rng">新版 L${h.new_start}–${h.new_start+Math.max(h.new_count-1,0)}</span>`;
    const hc = S.data.comments.filter(c=>c.target==='hunk'&&c.file===f.path&&c.hunk===hi&&(S.showResolved||c.status!=='resolved'));
    html+=`<div class="card ${hi===S.activeHunk?'active':''}" data-hi="${hi}" data-lines="${esc(n.lines||'')}">
      <div class="head"><span class="hno">#${hi}</span>${n.title?`<span class="t">${inline(n.title)}</span>`:`<span class="t hint" style="font-weight:400">${esc(h.section||'')}</span>`}</div>
      <div class="chips">${rng}${(n.tags||[]).map(t=>`<span class="chip ${tagClass(t)}">${esc(t)}</span>`).join('')}${dgForHunk(f.path,hi).map(d=>`<span class="chip dgl" data-dg="${esc(d.id)}" title="${esc(d.title)}">${U(0xf1c5)} 图解</span>`).join('')}</div>
      <div class="body">${md(n.body)}</div>
      ${hc.length?`<div class="hc">${hc.map(threadHTML).join('')}</div>`:''}
      <div class="acts"><button class="sm ghost" data-act="hunkc">💬 对这段说明提意见</button></div></div>`;
  });
  html += `<div class="hint" style="margin-top:14px;line-height:1.9">点卡片 → 跳到代码并钉住高亮(<kbd>Esc</kbd> 取消) · 点带蓝边的代码行或 <b>#N</b> → 跳回卡片 · <kbd>v</kbd> 代码/图解/审查 · <kbd>/</kbd> 过滤文件 · <kbd>n</kbd> / <kbd>p</kbd> 上下一个 hunk · <kbd>[</kbd> / <kbd>]</kbd> 折叠左右栏(分隔条可拖宽) · 行首 <kbd>+</kbd> 评论该行 · 点文件头折叠</div>`;
  $('#notes').innerHTML = html;
  bindThreadActs($('#notes'));
  bindTabs();
  applyPin();
  $('#gen-add').onclick = () => { if($('#notes .gen .cform')) return; const fm=formEl('', async v=>{ await api('POST','/api/comments',{target:'general',body:v}); await refreshComments(); }, ()=>fm.remove()); $('#gen-add').before(fm); fm.querySelector('textarea').focus(); };
  $('#notes').querySelectorAll('.card[data-hi]').forEach(card=>{
    const hi=+card.dataset.hi;
    card.onclick = e => { if(e.target.closest('button')||e.target.closest('.cform')||e.target.closest('.thread')) return; showMid('code'); const fb=document.getElementById('f-'+S.active); fb.classList.remove('collapsed'); S.pin={fi:S.active,hi}; applyPin();
      // 说明带 lines 范围时滚到范围的第一行(大 hunk 里范围可能离 hunk 头很远), 否则滚到 hunk 头
      const t=document.getElementById(`h-${S.active}-${hi}`); const first=t && t.querySelector('tr.line.hl-pin'); const r=noteRange(S.active,hi);
      scrollCodeTo((first && r && !r.all) ? first : t, (first && r && !r.all) ? 0.2 : 0.05); };
    card.querySelector('[data-act=hunkc]').onclick = e => { e.stopPropagation(); showMid('code'); const hh=document.querySelector(`tr.hh[data-fi="${S.active}"][data-hi="${hi}"]`); scrollCodeTo(hh, 0.2); openForm({target:'hunk', fi:S.active, hi}, hh); };
    card.querySelectorAll('.dgl').forEach(c=>c.onclick=e=>{ e.stopPropagation(); scrollToDiagram(c.dataset.dg); });
    card.onmouseenter = () => highlightLines(S.active, hi, card.dataset.lines, true);
    card.onmouseleave = () => highlightLines(S.active, hi, card.dataset.lines, false);
  });
}
// 某个 hunk 的说明覆盖哪些行: 有 lines 就按 lines, 没有就是整个 hunk 的 +/- 行(返回 {side,a,b,all})
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
const dgSrc = dg => `${BASE}/api/diagram?i=${encodeURIComponent(dg.id)}&theme=${isDark()?'dark':'light'}&v=${S.state?.notes_v||0}-${S.state?.diff_v||0}`;
const hunkLabel = ref => { const m=/^(.+)#(\d+)$/.exec(ref); return m ? m[1].split('/').pop()+'#'+m[2] : ref; };
function dgForHunk(path, hi){ return (S.data.diagrams||[]).filter(d=>(d.hunks||[]).includes(`${path}#${hi}`)); }

const PRIO_CN={high:'高优',medium:'中优',low:'低优'};
function rvCounts(){ const c={high:0,medium:0,low:0,open:0,done:0,wontfix:0}; ((S.data.review||{}).items||[]).forEach(it=>{ const st=it.status||'open'; c[st]=(c[st]||0)+1; if(st==='open') c[it.priority]=(c[it.priority]||0)+1; }); return c; }
function renderMidbar(){
  const n=(S.data.diagrams||[]).length; const rc=rvCounts(); const hasRv=!!S.data.review;
  const cs=S.data.comments||[]; const cDone=cs.filter(c=>c.status==='resolved').length; const rTotal=((S.data.review||{}).items||[]).length; const rDone=rc.done+rc.wontfix;
  const badge=(done,total,red)=>`<span class="n" style="${total&&done===total?'background:var(--ok);color:#fff':(red?'background:var(--del-fg);color:#fff':'')}">${done}<span style="opacity:.6">/${total}</span></span>`;
  $('#midbar').innerHTML = `<button class="tab ${S.mid==='code'?'on':''}" data-mid="code" title="${cs.length} 条评论, 已解决 ${cDone}">代码${cs.length?badge(cDone,cs.length,false):''}</button><button class="tab ${S.mid==='diagrams'?'on':''}" data-mid="diagrams">图解<span class="n">${n}</span></button><button class="tab ${S.mid==='review'?'on':''}" data-mid="review" title="${hasRv?`已处理 ${rDone} / 共 ${rTotal} 项, 待处理 ${rc.open}`:'还没有审查报告'}">审查${hasRv?badge(rDone,rTotal,rc.high>0):''}</button><span class="sp"></span>${S.mid==='diagrams'?`<span class="hint">${S.data.d2?'点图放大 · 点 hunk 标签跳回代码':'d2 未安装, 只能显示 mermaid 图'}</span>`:''}${S.mid==='review'&&hasRv?`<span class="hint">点勾 = 标记已处理 · 点 文件:行 跳到代码 · agent 用 hudiff review done &lt;id&gt; 打钩</span>`:''}`;
  $('#midbar').querySelectorAll('.tab').forEach(b=>b.onclick=()=>showMid(b.dataset.mid));
  $('#diff').style.display = S.mid==='code'?'':'none';
  $('#diagrams').style.display = S.mid==='diagrams'?'':'none';
  $('#review').style.display = S.mid==='review'?'':'none';
}
function rvLoc(it){ if(!it.file) return ''; const f=it.file, dir=f.includes('/')?f.slice(0,f.lastIndexOf('/')+1):'', base=f.slice(dir.length); return `<span class="loc" data-id="${esc(it.id)}"><span class="d">${esc(dir)}</span>${esc(base)}${it.line?` <b>${it.side==='old'?'旧版 ':''}L${it.line}</b>`:''}${it.hunk!=null?` <span class="d">#${it.hunk}</span>`:''}</span>`; }
function mdDiffColor(html){ return html.replace(/<pre><code>([\s\S]*?)<\/code><\/pre>/g,(m,body)=>'<pre><code>'+body.split('\n').map(l=>l.startsWith('-')?`<span class="dl">${l}</span>`:l.startsWith('+')?`<span class="ad">${l}</span>`:l).join('\n')+'</code></pre>'); }
function renderReview(){
  const rv=S.data.review;
  if(!rv){ renderReviewRequest($('#review'), true); return; }
  const c=rvCounts(); const items=rv.items||[];
  const rounds=rv.rounds||[];
  let html=`<div class="rv-head"><h2>审查报告</h2><span class="chip">${esc(rv.author||'reviewer')}</span>${rounds.length>1?`<span class="chip">${rounds.length} 轮</span>`:''}<span class="hint">${esc((rv.updated||'').slice(0,16).replace('T',' '))} · 待处理 ${c.open} · 已处理 ${c.done} · 拒绝 ${c.wontfix}</span><button class="sm" id="rv-again">↻ 再审一次</button></div><div id="rv-req" style="display:none"></div>`;
  if(rounds.length>1){ html+=`<div class="rv-sum">${rounds.map(r=>`<div class="rnd"><span class="chip">第 ${r.round} 轮</span> <span class="hint">${esc(r.author||'')} ${esc(String(r.at||'').slice(5,16).replace('T',' '))} · 新增 ${r.added||0} 项</span><div>${md(r.summary||'')}</div></div>`).join('')}</div>`; }
  else if(rv.summary) html+=`<div class="rv-sum">${md(rv.summary)}</div>`;
  if((rv.strengths||[]).length) html+=`<h3 style="font-size:12px;color:var(--fg2);margin:0 0 4px">优点</h3><ul class="rv-good">${rv.strengths.map(x=>`<li>${inline(x)}</li>`).join('')}</ul>`;
  ['high','medium','low'].forEach(pr=>{
    const its=items.filter(it=>it.priority===pr); if(!its.length) return;
    const open=its.filter(it=>(it.status||'open')==='open').length;
    html+=`<div class="rv-sec"><span class="pr ${pr}">${PRIO_CN[pr]}</span><h3>${pr==='high'?'不能发布, 必须修改':pr==='medium'?'建议修改, 不影响发布':'风格与可读性, 可改可不改'}</h3><span class="cnt">${open}/${its.length} 待处理</span></div>`;
    its.forEach(it=>{ const st=it.status||'open';
      html+=`<div class="rvi ${pr} ${st}" id="rv-${esc(it.id)}" data-id="${esc(it.id)}">
        <div class="cb" title="${st==='open'?'标记已处理':'重开'}">${st==='done'?'✓':st==='wontfix'?'–':''}</div>
        <div class="t">${inline(it.title)}<span class="id">${esc(it.id)}</span>${rounds.length>1&&it.round?`<span class="chip" title="第 ${it.round} 轮提出">R${it.round}</span>`:''}${st==='wontfix'?'<span class="chip" style="background:var(--del-bg);color:var(--del-fg)">已拒绝</span>':''}</div>
        ${rvLoc(it)}
        ${it.snippet?`<div class="snip">${esc(it.snippet)}</div>`:''}
        ${it.body?`<div class="body">${md(it.body)}</div>`:''}
        ${it.suggestion?`<div class="sug"><span class="k">修改建议</span>${mdDiffColor(md(it.suggestion))}</div>`:''}
        ${it.resolution?`<div class="res">${st==='wontfix'?'拒绝理由':'处理结果'}(${esc(it.resolved_by||'')}): ${inline(it.resolution)}</div>`:''}
        ${(it.notes||[]).length?`<div class="notes">${it.notes.map(n=>`<div class="note ${esc(n.verdict||'info')}"><span class="v">${n.verdict==='ok'?'✓ 复核通过':n.verdict==='disagree'?'⚠ 有异议':'· 说明'}</span><span class="w">${esc(n.who||'')} · R${n.round||'?'}</span><div class="b">${md(n.body)}</div></div>`).join('')}</div>`:''}
        <div class="acts">${st==='open'?`<button class="sm" data-act="done">已处理</button><button class="sm reject" data-act="wontfix">拒绝</button>`:`<button class="sm ghost" data-act="reopen">重开</button>`}<button class="sm ghost" data-act="note">补充说明</button></div>
      </div>`;
    });
  });
  $('#review').innerHTML=html;
  $('#rv-again').onclick=()=>{ const box=$('#rv-req'); if(box.style.display==='none'){ box.style.display=''; renderReviewRequest(box, false); } else box.style.display='none'; };
  const act=async(id,st)=>{ let text=''; if(st!=='open'){ text=await dlg.prompt(st==='done'?'标记为已处理':'拒绝这一项', st==='done'?'写一句做了什么(可空):':'写一句拒绝的理由(可空):', {ok:st==='done'?'已处理':'拒绝', danger:st==='wontfix', placeholder:'可选'}); if(text===null) return; } await api('PATCH','/api/review/'+id,{status:st,resolution:text,by:'user'}); await refreshReview(); };
  $('#review').querySelectorAll('.rvi').forEach(el=>{
    const id=el.dataset.id; const it=items.find(x=>x.id===id);
    el.querySelector('.cb').onclick=()=>act(id,(it.status||'open')==='open'?'done':'open');
    el.querySelectorAll('.acts button').forEach(b=>b.onclick=async()=>{ if(b.dataset.act==='note'){ const t=await dlg.prompt('补充说明', '写给复审 agent 或自己看的备注, 会显示在这一项下面:', {ok:'添加', placeholder:'比如: 这条拒绝是因为…'}); if(!t||!t.trim()) return; await api('POST',`/api/review/${id}/note`,{body:t,who:'user',verdict:'info'}); await refreshReview(); return; } act(id,{done:'done',wontfix:'wontfix',reopen:'open'}[b.dataset.act]); });
    const loc=el.querySelector('.loc'); if(loc) loc.onclick=()=>jumpToReviewItem(id);
  });
  const io=new IntersectionObserver(es=>{ es.forEach(en=>{ if(en.isIntersecting){ document.querySelectorAll('.rvo').forEach(o=>o.classList.toggle('active', o.dataset.id===en.target.dataset.id)); } }); }, {root:$('#review'), rootMargin:'-10% 0px -70% 0px', threshold:0});
  document.querySelectorAll('.rvi').forEach(f=>io.observe(f));
}
async function refreshReview(){ const d=await api('GET','/api/session'); S.data.review=d.review; S.state=d.state; renderMidbar(); renderReview(); markReviewLines(); if(S.mid==='review') renderNotes(); }
// 审查请求: 列出在跑的 agent, 选一个把「/hudiff review <id>」送过去(在 tmux 里就直接打进去, 否则给命令让用户粘贴)
async function renderReviewRequest(box, empty){
  const sid=S.data.session.id; const rvw=S.data.session.reviewer;
  box.innerHTML=`<div class="rvreq"><div class="rvreq-h">${empty?'<h2>还没有审查报告</h2>':''}<p>选一个<b>不是</b>写讲解的 agent 来交叉审查: 它会重新读改动、说明和图解, 写出总体评价、优点和高 / 中 / 低待处理项。报告写入后这里自动刷新。</p></div><div class="rvreq-list"><span class="hint">读取 agent 列表…</span></div><div class="rvreq-cmd"><span class="hint">或者手动在任意 agent 里输入:</span><code id="rvcmd">/hudiff review ${esc(sid)}</code><button class="sm ghost" id="rvcopy">复制</button></div></div>`;
  box.querySelector('#rvcopy').onclick=async()=>{ try{ await navigator.clipboard.writeText(`/hudiff review ${sid}`); toast('已复制'); }catch(e){ toast('复制失败, 手动选中吧'); } };
  let r; try{ r=await api('GET','/api/agents'); }catch(e){ box.querySelector('.rvreq-list').innerHTML='<span class="hint">读取失败</span>'; return; }
  const list=r.agents||[]; const same=list.filter(a=>a.same_repo), other=list.filter(a=>!a.same_repo);
  const name=a=> a.label && a.label!==a.kind ? `<span class="k">${esc(a.kind)} ·</span> ${esc(a.label)}` : esc(a.kind);
  const row=a=>`<div class="ag ${a.same_repo?'':'dim'}" data-pid="${a.pid}" data-pane="${esc(a.pane||'')}" data-kind="${esc(a.kind)}" data-label="${esc(a.label)}"><span class="st idle"></span><span class="nm"><span>${name(a)}</span>${a.waiting?'<span class="chip" title="它在等这个会话的评论, 大概就是写讲解的那个">讲解中</span>':''}${rvw&&rvw.pid===a.pid?'<span class="chip" style="background:var(--accent-bg);color:var(--accent)">上次审查</span>':''}</span><span class="br">${esc(a.branch)}</span><span class="cw" title="${esc(a.cwd)}">${esc(((c)=>c.length>40?'…'+c.slice(-39):c)(a.cwd_short||a.cwd))}</span></div>`;
  box.querySelector('.rvreq-list').innerHTML = (same.length?`<div class="sec">这个仓库</div>${same.map(row).join('')}`:'')+(other.length?`<div class="sec">其它</div>${other.map(row).join('')}`:'')+(!list.length?`<div class="empty">没找到在跑的 agent</div>`:'');
  box.querySelectorAll('.ag').forEach(el=>el.onclick=async()=>{
    const prev=S.data.review; const ok=await dlg.confirm(`让 ${el.dataset.label} 来审查?`, `会给它发一句「/hudiff review ${sid}」。它会重新读改动、说明、图解和评论${prev?`, 以及上一轮的 ${(prev.items||[]).length} 条审查项和你的处理结果, 新一轮会并进来而不是覆盖`:''}, 写出审查报告; 期间它手头的活会被打断。`, {ok:'发送审查请求'});
    if(!ok) return;
    const res=await api('POST','/api/review-request',{pid:+el.dataset.pid, pane:el.dataset.pane||null, kind:el.dataset.kind, label:el.dataset.label});
    if(res.delivered) toast(`审查请求已发给 ${esc(el.dataset.label)}, 报告写入后这里自动出现`);
    else await dlg.confirm('这个 agent 不在 tmux 里, 没法直接送过去', `在它的对话里粘贴这一句:\n\n/hudiff review ${sid}`, {ok:'知道了', cancel:'关闭'});
    const d=await api('GET','/api/session'); S.data.session=d.session;
  });
}
function jumpToReviewItem(id){
  const it=((S.data.review||{}).items||[]).find(x=>x.id===id); if(!it||!it.file) return;
  const fi=S.data.diff.files.findIndex(f=>f.path===it.file); if(fi<0){ toast('文件不在当前 diff 里'); return; }
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
    tr.classList.add('rv-'+it.priority); tr.dataset.rv=it.id; tr.title=`[${PRIO_CN[it.priority]}] ${it.title} — 点行号看审查项`; });
}
function renderReviewOutline(tabs){
  const rv=S.data.review; const c=rvCounts();
  let html=tabs+`<h3>审查项 · 待处理 ${c.open}</h3>`;
  if(!rv){ html+=`<div class="hint">还没有审查报告</div>`; $('#notes').innerHTML=html; bindTabs(); return; }
  html+=`<div class="rv-stats"><span class="chip pr high">高 ${c.high}</span><span class="chip pr medium">中 ${c.medium}</span><span class="chip pr low">低 ${c.low}</span><span class="chip">已处理 ${c.done}</span><span class="chip">拒绝 ${c.wontfix}</span></div>`;
  (rv.items||[]).forEach(it=>{ const dis=(it.notes||[]).some(n=>n.verdict==='disagree'); html+=`<div class="rvo ${it.priority} ${it.status||'open'}" data-id="${esc(it.id)}">${dis?'<span title="复核有异议" style="color:#bc4c00">⚠</span>':''}<span class="t">${esc(it.title)}</span><span class="l">${it.file?esc(it.file.split('/').pop())+(it.line?':'+it.line:''):''}</span></div>`; });
  html+=`<div class="hint" style="margin-top:14px;line-height:1.9">点一项定位到报告里 · 报告里点 文件:行 跳代码 · 代码区行号左侧的红/黄/蓝边 = 有审查项</div>`;
  $('#notes').innerHTML=html; bindTabs();
  $('#notes').querySelectorAll('.rvo').forEach(o=>o.onclick=()=>{ const el=document.getElementById('rv-'+o.dataset.id); if(el){ el.scrollIntoView({block:'center'}); el.classList.remove('locate'); void el.offsetWidth; el.classList.add('locate'); } });
}
function showMid(m){ if(S.mid===m) return; S.mid=m; LS.set('mid',m); renderMidbar(); renderNotes(); }

function renderDiagrams(){
  const dgs=S.data.diagrams||[];
  if(!dgs.length){ $('#diagrams').innerHTML = `<div class="hint" style="text-align:center;padding:60px 0">还没有图解。${S.data.d2?'agent 写说明时会按需附上时序图 / 流程图 / 类图; 改动地图在 diff 有符号级变化时自动出现。':'server 所在机器没装 d2(brew install d2), 装完任何 hudiff 命令会自动重启 server。'}</div>`; return; }
  let html='';
  dgs.forEach(dg=>{
    const chips=(dg.hunks||[]).map(r=>`<span class="chip hk" data-ref="${esc(r)}" title="${esc(r)}">${esc(hunkLabel(r))}</span>`).join('');
    const body = dg.kind==='mermaid' ? `<div class="svgwrap mm" data-id="${esc(dg.id)}"><span class="hint">渲染 mermaid…</span></div>` : `<div class="svgwrap"><img src="${dgSrc(dg)}" alt="${esc(dg.title)}"></div>`;
    html+=`<figure class="dg ${dg.id===S.activeDg?'active':''}" id="dg-${esc(dg.id)}" data-id="${esc(dg.id)}" style="margin:0 0 18px">
      <div class="dgh"><span class="t">${esc(dg.title)}</span><span class="kind">${esc(dg.kind)}${dg.auto?' · 自动':''}</span>${chips}${dg.src?`<button class="sm ghost" data-act="src">源码</button>`:''}<button class="sm ghost" data-act="full">⛶ 放大</button></div>
      ${body}
      ${dg.caption?`<figcaption>${md(dg.caption)}</figcaption>`:''}
      ${dg.src?`<pre class="src" style="display:none">${esc(dg.src)}</pre>`:''}</figure>`;
  });
  $('#diagrams').innerHTML = html;
  $('#diagrams').querySelectorAll('.hk').forEach(c=>c.onclick=()=>{ const m=/^(.+)#(\d+)$/.exec(c.dataset.ref); if(!m) return; const fi=S.data.diff.files.findIndex(f=>f.path===m[1]); if(fi<0) return; showMid('code'); const fb=document.getElementById('f-'+fi); fb.classList.remove('collapsed'); scrollCodeTo(document.getElementById(`h-${fi}-${m[2]}`), 0.05); setActive(fi,+m[2]); S.pin={fi,hi:+m[2]}; applyPin(); });
  $('#diagrams').querySelectorAll('[data-act=src]').forEach(b=>b.onclick=()=>{ const pre=b.closest('.dg').querySelector('pre.src'); pre.style.display = pre.style.display==='none'?'':'none'; });
  $('#diagrams').querySelectorAll('[data-act=full], .svgwrap').forEach(el=>el.onclick=e=>{ if(e.target.closest('button')&&el.tagName!=='BUTTON') return; const fig=el.closest('.dg'); const img=fig.querySelector('img'); const svg=fig.querySelector('svg'); openLightbox(fig.querySelector('.t').textContent, img?img.src:(svg?'data:image/svg+xml;charset=utf-8,'+encodeURIComponent(svg.outerHTML):null)); });
  renderMermaid();
  const io=new IntersectionObserver(es=>{ es.forEach(en=>{ if(en.isIntersecting){ S.activeDg=en.target.dataset.id; document.querySelectorAll('.dg').forEach(f=>f.classList.toggle('active', f.dataset.id===S.activeDg)); document.querySelectorAll('.dgcard').forEach(c=>c.classList.toggle('active', c.dataset.id===S.activeDg)); } }); }, {root:$('#diagrams'), rootMargin:'-20% 0px -60% 0px', threshold:0});
  document.querySelectorAll('.dg').forEach(f=>io.observe(f));
}
let mermaidLoading=null;
function renderMermaid(){
  const boxes=[...document.querySelectorAll('.svgwrap.mm')]; if(!boxes.length) return;
  const go=async()=>{ mermaid.initialize({startOnLoad:false, theme:isDark()?'dark':'default', securityLevel:'loose'});
    for(const box of boxes){ const dg=(S.data.diagrams||[]).find(d=>d.id===box.dataset.id); try{ const {svg}=await mermaid.render('mm-'+Math.random().toString(36).slice(2), dg.src); box.innerHTML=svg; }catch(e){ box.innerHTML=`<pre class="hint" style="text-align:left;color:var(--del-fg)">mermaid 渲染失败:\n${esc(e.message||e)}</pre>`; } } };
  if(window.mermaid) return go();
  if(!mermaidLoading){ mermaidLoading=new Promise((res,rej)=>{ const sc=document.createElement('script'); sc.src='https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js'; sc.onload=res; sc.onerror=rej; document.head.appendChild(sc); }); }
  mermaidLoading.then(go).catch(()=>boxes.forEach(b=>b.innerHTML='<span class="hint">mermaid 加载失败(需要联网)</span>'));
}
function scrollToDiagram(id){ showMid('diagrams'); const f=document.getElementById('dg-'+id); if(f){ f.scrollIntoView({block:'start'}); S.activeDg=id; document.querySelectorAll('.dg').forEach(x=>x.classList.toggle('active', x.dataset.id===id)); } }

// 灯箱: 滚轮缩放 + 拖动
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
  let html = tabs + `<h3>图列表 · ${dgs.length} 张</h3>`;
  if(!dgs.length) html+=`<div class="hint">还没有图</div>`;
  dgs.forEach(dg=>{ html+=`<div class="dgcard ${dg.id===S.activeDg?'active':''}" data-id="${esc(dg.id)}"><div class="th">${dg.kind==='mermaid'?'<span class="hint">mermaid</span>':`<img src="${dgSrc(dg)}" alt="">`}</div><div class="tt"><span>${esc(dg.title)}</span><span class="hint">${dg.auto?'自动':(dg.hunks||[]).length+' 个 hunk'}</span></div></div>`; });
  html += `<div class="hint" style="margin-top:14px;line-height:1.9">点缩略图定位到大图 · 大图上的 hunk 标签跳回代码 · <kbd>v</kbd> 切换 代码/图解</div>`;
  $('#notes').innerHTML = html;
  bindTabs();
  $('#notes').querySelectorAll('.dgcard').forEach(c=>c.onclick=()=>scrollToDiagram(c.dataset.id));
}

// ── 发给谁: 有 wait 就叫醒; 没有就投递到绑定的 Claude Code pane; 都没有就弹列表选 ──
const AG={list:[], waiting:0, bound:null, alive:false};
function renderSendTarget(){
  const el=$('#send-tgt'); if(!el) return;
  if(AG.waiting) el.innerHTML=`<span class="dot"></span>${AG.waiting} 个 agent 在等`;
  else if(AG.bound) el.innerHTML=`<span class="dot ${AG.alive?'':'off'}"></span>→ ${esc(AG.bound.label||AG.bound.kind||'agent')}${AG.alive?'':' (已退出)'}`;
  else el.innerHTML=`<span class="dot off"></span>未绑定 agent`;
}
async function loadAgents(){ const r=await api('GET','/api/agents'); AG.list=r.agents||[]; AG.waiting=r.waiting||0; AG.bound=r.bound; AG.alive=!!AG.list.find(a=>a.bound); renderSendTarget(); return r; }
function toggleAgentPop(force){
  const pop=$('#agentpop'); const on = force!==undefined ? force : !pop.classList.contains('on');
  if(!on){ pop.classList.remove('on'); return; }
  pop.innerHTML=`<div class="hd">正在读取 agent 列表…</div>`; pop.classList.add('on');
  loadAgents().then(()=>{
    const same=AG.list.filter(a=>a.same_repo), other=AG.list.filter(a=>!a.same_repo);
    const name=a=> a.label && a.label!==a.kind ? `<span class="k">${esc(a.kind)} ·</span> ${esc(a.label)}` : esc(a.kind);
    const row=a=>`<div class="ag ${a.same_repo?'':'dim'}" data-pid="${a.pid}" data-pane="${esc(a.pane||'')}" data-kind="${esc(a.kind)}" data-label="${esc(a.label)}"><span class="st ${a.waiting?'waiting':'idle'}"></span><span class="nm"><span>${name(a)}</span>${a.waiting?'<span class="chip" style="background:var(--del-bg);color:var(--del-fg)">在等</span>':''}${a.bound?'<span class="chip" style="background:var(--accent-bg);color:var(--accent)">已绑定</span>':''}</span><span class="br">${esc(a.branch)}</span><span class="cw" title="${esc(a.cwd)}">${esc(((c)=>c.length>40?'…'+c.slice(-39):c)(a.cwd_short||a.cwd))}</span></div>`;
    pop.innerHTML=`<button class="sm ghost rf" id="ag-refresh" title="刷新">↻</button>`
      +(same.length?`<div class="sec">这个仓库</div>${same.map(row).join('')}`:'')
      +(other.length?`<div class="sec">其它</div>${other.map(row).join('')}`:'')
      +(!AG.list.length?`<div class="empty">没找到在跑的 agent(支持 claude / pi / opencode / codex / gemini / aider 等)</div>`:'');
    $('#ag-refresh').onclick=e=>{ e.stopPropagation(); toggleAgentPop(true); };
    pop.querySelectorAll('.ag').forEach(el=>el.onclick=async e=>{ e.stopPropagation(); pop.classList.remove('on'); const unsent=S.data.comments.filter(c=>c.status==='open').length; await doSubmit({pid:+el.dataset.pid, pane:el.dataset.pane||null, kind:el.dataset.kind, label:el.dataset.label, bind_only:!unsent}); });
  });
}
document.addEventListener('click', e=>{ const pop=$('#agentpop'); if(pop && pop.classList.contains('on') && !e.target.closest('.sendgrp')) pop.classList.remove('on'); });
async function doSubmit(opts){
  if(!opts){
    await loadAgents();
    if(!AG.waiting && !(AG.bound && AG.alive)){ toggleAgentPop(true); toast('没有 agent 在等, 先选一个接收的 agent'); return; }
    opts={};
  }
  const r=await api('POST','/api/submit',opts);
  if(opts.bind_only){ toast(`已绑定 ${opts.label}, 之后点「发给 agent」直接发给它`); await loadAgents(); return; }
  if(r.delivered==='wait') toast(`已发送 ${r.sent} 条, ${r.waiting} 个 agent 在等, 已叫醒`);
  else if(r.delivered==='pane') toast(`已发送 ${r.sent} 条, 已通知 ${esc(r.label||'agent')}`);
  else toast(`已标记 ${r.sent} 条为待处理${r.error?'; '+r.error:' —— 点 ▾ 选一个接收的 agent'}`);
  await refreshComments(); await loadAgents();
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
  if(c.target==='line') return `${c.side==='new'?'新版':'旧版'} L${c.line}`;
  if(c.target==='hunk') return `整段 #${c.hunk}`;
  return '整体意见';
}
function renderCommentList(tabs){
  const files=S.data.diff.files; const order=Object.fromEntries(files.map((f,i)=>[f.path,i]));
  const all=S.data.comments.slice().sort((a,b)=>{ const fa=a.file?order[a.file]??999:-1, fb=b.file?order[b.file]??999:-1; if(fa!==fb) return fa-fb; if((a.hunk??-1)!==(b.hunk??-1)) return (a.hunk??-1)-(b.hunk??-1); return (a.line??0)-(b.line??0); });
  const counts={}; all.forEach(c=>counts[c.status]=(counts[c.status]||0)+1);
  const q=C.q.trim().toLowerCase();
  const list=all.filter(c=>C.st.has(c.status)).filter(c=>!q || (c.body+' '+(c.file||'')+' '+(c.snippet||'')+' '+(c.replies||[]).map(r=>r.body).join(' ')).toLowerCase().includes(q));
  const stcn={open:'未发送',sent:'待处理',replied:'已回复',resolved:'已解决'};
  let html = tabs + `<div class="cbar"><div class="q"><span class="nf">${U(0xf002)}</span><input id="cq" placeholder="过滤评论…" value="${esc(C.q)}"></div></div>
    <div class="cbar">${['open','sent','replied','resolved'].map(st=>`<span class="chip ${st} ${C.st.has(st)?'':'off'}" data-st="${st}">${stcn[st]} ${counts[st]||0}</span>`).join('')}</div>`;
  if(!list.length) html += `<div class="hint" style="text-align:center;padding:30px 0">${all.length?'没有匹配的评论':'还没有评论 —— 在代码行上点 + 开始'}</div>`;
  list.forEach(c=>{
    const f=c.file||''; const dir=f.includes('/')?f.slice(0,f.lastIndexOf('/')+1):''; const base=f.slice(dir.length);
    let snip=''; if(c.target==='line'){ const [,ln]=find(c); const t=ln?ln.t:(c.snippet!=null?' ':null); const txt=ln?ln.s:c.snippet; if(txt!=null) snip=`<div class="snip ${t==='+'?'add':t==='-'?'del':''}">${t===' '?' ':(t||' ')} ${esc(txt)}</div>`; }
    else if(c.target==='hunk'){ const fi=order[c.file]; const h=files[fi]?.hunks[c.hunk]; if(h) snip=`<div class="snip">${esc(h.header)}</div>`; }
    const last=(c.replies||[]).at(-1);
    html += `<div class="ccard ${c.status}" data-id="${c.id}"><div class="ch">${f?`<span class="f" title="${esc(f)}"><span class="d">${esc(dir)}</span>${esc(base)}</span>`:`<span class="f">整体意见</span>`}<span class="ln">${cmtLoc(c)}</span><span class="chip ${c.status}">${stcn[c.status]}</span>${c.stale?'<span class="chip stale">⚠</span>':''}</div>${snip}<div class="cb">${inline(c.body)}</div>
      <div class="cf"><span class="last">${last?`<b>${last.who==='agent'?'agent':'我'}:</b> ${esc(last.body.split('\n')[0])}`:'没有回复'}</span><span>${(c.replies||[]).length?(c.replies.length+' 条回复'):''}</span><span class="hint">${c.id}</span></div></div>`;
  });
  $('#notes').innerHTML = html;
  bindTabs();
  $('#cq').oninput = e => { C.q=e.target.value; const pos=e.target.selectionStart; renderCommentList(tabs); const i=$('#cq'); i.focus(); i.setSelectionRange(pos,pos); };
  $('#cq').onkeydown = e => e.stopPropagation();
  $('#notes').querySelectorAll('.cbar .chip[data-st]').forEach(ch=>ch.onclick=()=>{ const st=ch.dataset.st; if(C.st.has(st)) C.st.delete(st); else C.st.add(st); renderCommentList(tabs); });
  $('#notes').querySelectorAll('.ccard').forEach(el=>el.onclick=()=>jumpToComment(el.dataset.id));
}
function find(c){ const fi=S.data.diff.files.findIndex(f=>f.path===c.file); const h=S.data.diff.files[fi]?.hunks[c.hunk]; if(!h) return [null,null]; const ln=h.lines.find(l=>(c.side==='new'?l.n:l.o)===c.line && !(c.side==='new'&&l.t==='-') && !(c.side==='old'&&l.t==='+')); return [h,ln||null]; }
function jumpToComment(id){
  const c=S.data.comments.find(x=>x.id===id); if(!c) return;
  if(c.status==='resolved' && !S.showResolved){ S.showResolved=true; $('#showres').checked=true; renderComments(); }
  const flash=()=>{ const th=document.querySelector(`.thread[data-id="${id}"]`); if(th){ th.classList.remove('flash'); void th.offsetWidth; th.classList.add('flash'); th.scrollIntoView({block:'center'}); } };
  if(c.target==='general'){ S.tab='notes'; LS.set('tab','notes'); renderNotes(); flash(); return; }
  const fi=S.data.diff.files.findIndex(f=>f.path===c.file); if(fi<0){ toast('这条评论对应的文件不在当前 diff 里'); return; }
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

document.addEventListener('keydown', e => {
  if(e.target.tagName==='TEXTAREA'||e.target.tagName==='INPUT') return;
  if(e.key==='/'){ e.preventDefault(); if($('#main').classList.contains('nav-off')) $('#tog-l').click(); $('#q').focus(); return; }
  if(e.key==='['){ $('#tog-l').click(); return; } if(e.key===']'){ $('#tog-r').click(); return; }
  if(e.key==='Escape' && $('#lb').style.display!=='none'){ closeLightbox(); return; }
  if(e.key==='0' && $('#lb').style.display!=='none'){ $('#lb-img').onload(); return; }
  if(e.key==='v'){ showMid({code:'diagrams',diagrams:'review',review:'code'}[S.mid]); return; }
  if(e.key==='Escape' && S.pin){ S.pin=null; applyPin(); return; }
  if(e.key!=='n'&&e.key!=='p') return;
  const all=[...document.querySelectorAll('table.hunk')].filter(t=>!t.closest('.collapsed')); if(!all.length) return;
  let idx=all.findIndex(t=>+t.dataset.fi===S.active&&+t.dataset.hi===S.activeHunk);
  idx = e.key==='n' ? Math.min(idx+1, all.length-1) : Math.max(idx-1, 0);
  showMid('code'); scrollCodeTo(all[idx], 0.05); setActive(+all[idx].dataset.fi, +all[idx].dataset.hi);
});

// 轮询: agent 写了说明 / 别处改了评论 → 局部刷新
setInterval(async () => {
  if(document.hidden) return;
  try{
    const r = await fetch(BASE+'/api/state'); if(!r.ok) throw 0; const st = await r.json();
    if($('#offline').style.display!=='none'){ $('#offline').style.display='none'; toast('已重新连上'); await loadAll(); return; }
    if(!S.state) return;
    if(st.diff_v!==S.state.diff_v){ await loadAll(); toast('diff 已刷新'); return; }
    if(st.notes_v!==S.state.notes_v){ const d=await api('GET','/api/session'); S.data.notes=d.notes; S.data.diagrams=d.diagrams; S.state=d.state; renderHeader(); renderDiff(); renderComments(); renderMidbar(); renderDiagrams(); markReviewLines(); renderNotes(); toast('说明已更新'); return; }
    if(st.waiting!==AG.waiting || st.bound_alive!==AG.alive){ AG.waiting=st.waiting||0; AG.alive=!!st.bound_alive; renderSendTarget(); }
    if(st.review_v!==S.state.review_v){ await refreshReview(); toast('审查报告已更新'); return; }
    if(st.comments_v!==S.state.comments_v){ await refreshComments(); }
  }catch(e){ $('#offline').style.display='block'; }
}, 5000);

loadAll().then(()=>{ const m=/^#f-(\d+)$/.exec(location.hash); if(m){ const el=document.getElementById('f-'+m[1]); if(el){ el.scrollIntoView({block:'start'}); setActive(+m[1]); } } });
