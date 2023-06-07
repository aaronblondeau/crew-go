import{c as z,O as K,P as Ce,R as D,S as Te,r as q,a as b,h as T,i as B,T as ue,U as se,l as Ee,b as re,V as Se,W as Z,X as pe,Y as G,w as k,o as Be,k as Q,Z as He,x as $,_ as J,$ as Le,B as Me,a0 as Pe,d as We,a1 as Re,a2 as Oe,a3 as Ae,a4 as $e,a5 as Fe,a6 as ze,a7 as Ke,a8 as De,a9 as Qe,aa as ee,v as je,ab as Ie,ac as Ne,ad as Ve,ae as Ue,af as Xe}from"./index.dbea19db.js";var it=z({name:"QItem",props:{...K,...Ce,tag:{type:String,default:"div"},active:{type:Boolean,default:null},clickable:Boolean,dense:Boolean,insetLevel:Number,tabindex:[String,Number],focused:Boolean,manualFocus:Boolean},emits:["click","keyup"],setup(e,{slots:n,emit:a}){const{proxy:{$q:t}}=B(),l=D(e,t),{hasLink:c,linkAttrs:o,linkClass:d,linkTag:f,navigateOnClick:s}=Te(),r=q(null),h=q(null),g=b(()=>e.clickable===!0||c.value===!0||e.tag==="label"),i=b(()=>e.disable!==!0&&g.value===!0),m=b(()=>"q-item q-item-type row no-wrap"+(e.dense===!0?" q-item--dense":"")+(l.value===!0?" q-item--dark":"")+(c.value===!0&&e.active===null?d.value:e.active===!0?` q-item--active${e.activeClass!==void 0?` ${e.activeClass}`:""}`:"")+(e.disable===!0?" disabled":"")+(i.value===!0?" q-item--clickable q-link cursor-pointer "+(e.manualFocus===!0?"q-manual-focusable":"q-focusable q-hoverable")+(e.focused===!0?" q-manual-focusable--focused":""):"")),H=b(()=>{if(e.insetLevel===void 0)return null;const v=t.lang.rtl===!0?"Right":"Left";return{["padding"+v]:16+e.insetLevel*56+"px"}});function L(v){i.value===!0&&(h.value!==null&&(v.qKeyEvent!==!0&&document.activeElement===r.value?h.value.focus():document.activeElement===h.value&&r.value.focus()),s(v))}function E(v){if(i.value===!0&&ue(v,13)===!0){se(v),v.qKeyEvent=!0;const w=new MouseEvent("click",v);w.qKeyEvent=!0,r.value.dispatchEvent(w)}a("keyup",v)}function M(){const v=Ee(n.default,[]);return i.value===!0&&v.unshift(T("div",{class:"q-focus-helper",tabindex:-1,ref:h})),v}return()=>{const v={ref:r,class:m.value,style:H.value,role:"listitem",onClick:L,onKeyup:E};return i.value===!0?(v.tabindex=e.tabindex||"0",Object.assign(v,o.value)):g.value===!0&&(v["aria-disabled"]="true"),T(f.value,v,M())}}}),ut=z({name:"QList",props:{...K,bordered:Boolean,dense:Boolean,separator:Boolean,padding:Boolean,tag:{type:String,default:"div"}},setup(e,{slots:n}){const a=B(),t=D(e,a.proxy.$q),l=b(()=>"q-list"+(e.bordered===!0?" q-list--bordered":"")+(e.dense===!0?" q-list--dense":"")+(e.separator===!0?" q-list--separator":"")+(t.value===!0?" q-list--dark":"")+(e.padding===!0?" q-list--padding":""));return()=>T(e.tag,{class:l.value},re(n.default))}});function Ye(){if(window.getSelection!==void 0){const e=window.getSelection();e.empty!==void 0?e.empty():e.removeAllRanges!==void 0&&(e.removeAllRanges(),Se.is.mobile!==!0&&e.addRange(document.createRange()))}else document.selection!==void 0&&document.selection.empty()}const _e={target:{default:!0},noParentEvent:Boolean,contextMenu:Boolean};function Ze({showing:e,avoidEmit:n,configureAnchorEl:a}){const{props:t,proxy:l,emit:c}=B(),o=q(null);let d=null;function f(i){return o.value===null?!1:i===void 0||i.touches===void 0||i.touches.length<=1}const s={};a===void 0&&(Object.assign(s,{hide(i){l.hide(i)},toggle(i){l.toggle(i),i.qAnchorHandled=!0},toggleKey(i){ue(i,13)===!0&&s.toggle(i)},contextClick(i){l.hide(i),Z(i),pe(()=>{l.show(i),i.qAnchorHandled=!0})},prevent:Z,mobileTouch(i){if(s.mobileCleanup(i),f(i)!==!0)return;l.hide(i),o.value.classList.add("non-selectable");const m=i.target;G(s,"anchor",[[m,"touchmove","mobileCleanup","passive"],[m,"touchend","mobileCleanup","passive"],[m,"touchcancel","mobileCleanup","passive"],[o.value,"contextmenu","prevent","notPassive"]]),d=setTimeout(()=>{d=null,l.show(i),i.qAnchorHandled=!0},300)},mobileCleanup(i){o.value.classList.remove("non-selectable"),d!==null&&(clearTimeout(d),d=null),e.value===!0&&i!==void 0&&Ye()}}),a=function(i=t.contextMenu){if(t.noParentEvent===!0||o.value===null)return;let m;i===!0?l.$q.platform.is.mobile===!0?m=[[o.value,"touchstart","mobileTouch","passive"]]:m=[[o.value,"mousedown","hide","passive"],[o.value,"contextmenu","contextClick","notPassive"]]:m=[[o.value,"click","toggle","passive"],[o.value,"keyup","toggleKey","passive"]],G(s,"anchor",m)});function r(){He(s,"anchor")}function h(i){for(o.value=i;o.value.classList.contains("q-anchor--skip");)o.value=o.value.parentNode;a()}function g(){if(t.target===!1||t.target===""||l.$el.parentNode===null)o.value=null;else if(t.target===!0)h(l.$el.parentNode);else{let i=t.target;if(typeof t.target=="string")try{i=document.querySelector(t.target)}catch{i=void 0}i!=null?(o.value=i.$el||i,a()):(o.value=null,console.error(`Anchor: target "${t.target}" not found`))}}return k(()=>t.contextMenu,i=>{o.value!==null&&(r(),a(i))}),k(()=>t.target,()=>{o.value!==null&&r(),g()}),k(()=>t.noParentEvent,i=>{o.value!==null&&(i===!0?r():a())}),Be(()=>{g(),n!==!0&&t.modelValue===!0&&o.value===null&&c("update:modelValue",!1)}),Q(()=>{d!==null&&clearTimeout(d),r()}),{anchorEl:o,canShow:f,anchorEvents:s}}function Ge(e,n){const a=q(null);let t;function l(d,f){const s=`${f!==void 0?"add":"remove"}EventListener`,r=f!==void 0?f:t;d!==window&&d[s]("scroll",r,$.passive),window[s]("scroll",r,$.passive),t=f}function c(){a.value!==null&&(l(a.value),a.value=null)}const o=k(()=>e.noParentEvent,()=>{a.value!==null&&(c(),n())});return Q(o),{localScrollTarget:a,unconfigureScrollTarget:c,changeScrollEvent:l}}const{notPassiveCapture:S}=$,x=[];function p(e){const n=e.target;if(n===void 0||n.nodeType===8||n.classList.contains("no-pointer-events")===!0)return;let a=J.length-1;for(;a>=0;){const t=J[a].$;if(t.type.name!=="QDialog")break;if(t.props.seamless!==!0)return;a--}for(let t=x.length-1;t>=0;t--){const l=x[t];if((l.anchorEl.value===null||l.anchorEl.value.contains(n)===!1)&&(n===document.body||l.innerRef.value!==null&&l.innerRef.value.contains(n)===!1))e.qClickOutside=!0,l.onClickOutside(e);else return}}function Je(e){x.push(e),x.length===1&&(document.addEventListener("mousedown",p,S),document.addEventListener("touchstart",p,S))}function te(e){const n=x.findIndex(a=>a===e);n>-1&&(x.splice(n,1),x.length===0&&(document.removeEventListener("mousedown",p,S),document.removeEventListener("touchstart",p,S)))}let ne,le;function oe(e){const n=e.split(" ");return n.length!==2?!1:["top","center","bottom"].includes(n[0])!==!0?(console.error("Anchor/Self position must start with one of top/center/bottom"),!1):["left","middle","right","start","end"].includes(n[1])!==!0?(console.error("Anchor/Self position must end with one of left/middle/right/start/end"),!1):!0}function et(e){return e?!(e.length!==2||typeof e[0]!="number"||typeof e[1]!="number"):!0}const F={"start#ltr":"left","start#rtl":"right","end#ltr":"right","end#rtl":"left"};["left","middle","right"].forEach(e=>{F[`${e}#ltr`]=e,F[`${e}#rtl`]=e});function ae(e,n){const a=e.split(" ");return{vertical:a[0],horizontal:F[`${a[1]}#${n===!0?"rtl":"ltr"}`]}}function tt(e,n){let{top:a,left:t,right:l,bottom:c,width:o,height:d}=e.getBoundingClientRect();return n!==void 0&&(a-=n[1],t-=n[0],c+=n[1],l+=n[0],o+=n[0],d+=n[1]),{top:a,bottom:c,height:d,left:t,right:l,width:o,middle:t+(l-t)/2,center:a+(c-a)/2}}function nt(e,n,a){let{top:t,left:l}=e.getBoundingClientRect();return t+=n.top,l+=n.left,a!==void 0&&(t+=a[1],l+=a[0]),{top:t,bottom:t+1,height:1,left:l,right:l+1,width:1,middle:l,center:t}}function lt(e){return{top:0,center:e.offsetHeight/2,bottom:e.offsetHeight,left:0,middle:e.offsetWidth/2,right:e.offsetWidth}}function ie(e,n,a){return{top:e[a.anchorOrigin.vertical]-n[a.selfOrigin.vertical],left:e[a.anchorOrigin.horizontal]-n[a.selfOrigin.horizontal]}}function ot(e){if(Le.is.ios===!0&&window.visualViewport!==void 0){const d=document.body.style,{offsetLeft:f,offsetTop:s}=window.visualViewport;f!==ne&&(d.setProperty("--q-pe-left",f+"px"),ne=f),s!==le&&(d.setProperty("--q-pe-top",s+"px"),le=s)}const{scrollLeft:n,scrollTop:a}=e.el,t=e.absoluteOffset===void 0?tt(e.anchorEl,e.cover===!0?[0,0]:e.offset):nt(e.anchorEl,e.absoluteOffset,e.offset);let l={maxHeight:e.maxHeight,maxWidth:e.maxWidth,visibility:"visible"};(e.fit===!0||e.cover===!0)&&(l.minWidth=t.width+"px",e.cover===!0&&(l.minHeight=t.height+"px")),Object.assign(e.el.style,l);const c=lt(e.el);let o=ie(t,c,e);if(e.absoluteOffset===void 0||e.offset===void 0)A(o,t,c,e.anchorOrigin,e.selfOrigin);else{const{top:d,left:f}=o;A(o,t,c,e.anchorOrigin,e.selfOrigin);let s=!1;if(o.top!==d){s=!0;const r=2*e.offset[1];t.center=t.top-=r,t.bottom-=r+2}if(o.left!==f){s=!0;const r=2*e.offset[0];t.middle=t.left-=r,t.right-=r+2}s===!0&&(o=ie(t,c,e),A(o,t,c,e.anchorOrigin,e.selfOrigin))}l={top:o.top+"px",left:o.left+"px"},o.maxHeight!==void 0&&(l.maxHeight=o.maxHeight+"px",t.height>o.maxHeight&&(l.minHeight=l.maxHeight)),o.maxWidth!==void 0&&(l.maxWidth=o.maxWidth+"px",t.width>o.maxWidth&&(l.minWidth=l.maxWidth)),Object.assign(e.el.style,l),e.el.scrollTop!==a&&(e.el.scrollTop=a),e.el.scrollLeft!==n&&(e.el.scrollLeft=n)}function A(e,n,a,t,l){const c=a.bottom,o=a.right,d=Me(),f=window.innerHeight-d,s=document.body.clientWidth;if(e.top<0||e.top+c>f)if(l.vertical==="center")e.top=n[t.vertical]>f/2?Math.max(0,f-c):0,e.maxHeight=Math.min(c,f);else if(n[t.vertical]>f/2){const r=Math.min(f,t.vertical==="center"?n.center:t.vertical===l.vertical?n.bottom:n.top);e.maxHeight=Math.min(c,r),e.top=Math.max(0,r-c)}else e.top=Math.max(0,t.vertical==="center"?n.center:t.vertical===l.vertical?n.top:n.bottom),e.maxHeight=Math.min(c,f-e.top);if(e.left<0||e.left+o>s)if(e.maxWidth=Math.min(o,s),l.horizontal==="middle")e.left=n[t.horizontal]>s/2?Math.max(0,s-o):0;else if(n[t.horizontal]>s/2){const r=Math.min(s,t.horizontal==="middle"?n.middle:t.horizontal===l.horizontal?n.right:n.left);e.maxWidth=Math.min(o,r),e.left=Math.max(0,r-e.maxWidth)}else e.left=Math.max(0,t.horizontal==="middle"?n.middle:t.horizontal===l.horizontal?n.left:n.right),e.maxWidth=Math.min(o,s-e.left)}var st=z({name:"QMenu",inheritAttrs:!1,props:{..._e,...Pe,...K,...We,persistent:Boolean,autoClose:Boolean,separateClosePopup:Boolean,noRouteDismiss:Boolean,noRefocus:Boolean,noFocus:Boolean,fit:Boolean,cover:Boolean,square:Boolean,anchor:{type:String,validator:oe},self:{type:String,validator:oe},offset:{type:Array,validator:et},scrollTarget:{default:void 0},touchPosition:Boolean,maxHeight:{type:String,default:null},maxWidth:{type:String,default:null}},emits:[...Re,"click","escapeKey"],setup(e,{slots:n,emit:a,attrs:t}){let l=null,c,o,d;const f=B(),{proxy:s}=f,{$q:r}=s,h=q(null),g=q(!1),i=b(()=>e.persistent!==!0&&e.noRouteDismiss!==!0),m=D(e,r),{registerTick:H,removeTick:L}=Oe(),{registerTimeout:E}=Ae(),{transitionProps:M,transitionStyle:v}=$e(e),{localScrollTarget:w,changeScrollEvent:ce,unconfigureScrollTarget:de}=Ge(e,Y),{anchorEl:y,canShow:fe}=Ze({showing:g}),{hide:j}=Fe({showing:g,canShow:fe,handleShow:be,handleHide:xe,hideOnRouteChange:i,processOnMount:!0}),{showPortal:I,hidePortal:N,renderPortal:ve}=ze(f,h,ke,"menu"),P={anchorEl:y,innerRef:h,onClickOutside(u){if(e.persistent!==!0&&g.value===!0)return j(u),(u.type==="touchstart"||u.target.classList.contains("q-dialog__backdrop"))&&se(u),!0}},V=b(()=>ae(e.anchor||(e.cover===!0?"center middle":"bottom start"),r.lang.rtl)),he=b(()=>e.cover===!0?V.value:ae(e.self||"top start",r.lang.rtl)),me=b(()=>(e.square===!0?" q-menu--square":"")+(m.value===!0?" q-menu--dark q-dark":"")),ge=b(()=>e.autoClose===!0?{onClick:ye}:{}),U=b(()=>g.value===!0&&e.persistent!==!0);k(U,u=>{u===!0?(Ue(R),Je(P)):(ee(R),te(P))});function W(){Ve(()=>{let u=h.value;u&&u.contains(document.activeElement)!==!0&&(u=u.querySelector("[autofocus][tabindex], [data-autofocus][tabindex]")||u.querySelector("[autofocus] [tabindex], [data-autofocus] [tabindex]")||u.querySelector("[autofocus], [data-autofocus]")||u,u.focus({preventScroll:!0}))})}function be(u){if(l=e.noRefocus===!1?document.activeElement:null,Ke(_),I(),Y(),c=void 0,u!==void 0&&(e.touchPosition||e.contextMenu)){const O=De(u);if(O.left!==void 0){const{top:qe,left:we}=y.value.getBoundingClientRect();c={left:O.left-we,top:O.top-qe}}}o===void 0&&(o=k(()=>r.screen.width+"|"+r.screen.height+"|"+e.self+"|"+e.anchor+"|"+r.lang.rtl,C)),e.noFocus!==!0&&document.activeElement.blur(),H(()=>{C(),e.noFocus!==!0&&W()}),E(()=>{r.platform.is.ios===!0&&(d=e.autoClose,h.value.click()),C(),I(!0),a("show",u)},e.transitionDuration)}function xe(u){L(),N(),X(!0),l!==null&&(u===void 0||u.qClickOutside!==!0)&&(((u&&u.type.indexOf("key")===0?l.closest('[tabindex]:not([tabindex^="-"])'):void 0)||l).focus(),l=null),E(()=>{N(!0),a("hide",u)},e.transitionDuration)}function X(u){c=void 0,o!==void 0&&(o(),o=void 0),(u===!0||g.value===!0)&&(Qe(_),de(),te(P),ee(R)),u!==!0&&(l=null)}function Y(){(y.value!==null||e.scrollTarget!==void 0)&&(w.value=je(y.value,e.scrollTarget),ce(w.value,C))}function ye(u){d!==!0?(Ie(s,u),a("click",u)):d=!1}function _(u){U.value===!0&&e.noFocus!==!0&&Xe(h.value,u.target)!==!0&&W()}function R(u){a("escapeKey"),j(u)}function C(){const u=h.value;u===null||y.value===null||ot({el:u,offset:e.offset,anchorEl:y.value,anchorOrigin:V.value,selfOrigin:he.value,absoluteOffset:c,fit:e.fit,cover:e.cover,maxHeight:e.maxHeight,maxWidth:e.maxWidth})}function ke(){return T(Ne,M.value,()=>g.value===!0?T("div",{role:"menu",...t,ref:h,tabindex:-1,class:["q-menu q-position-engine scroll"+me.value,t.class],style:[t.style,v.value],...ge.value},re(n.default)):null)}return Q(X),Object.assign(s,{focus:W,updatePosition:C}),ve}});export{st as Q,ut as a,it as b,Ye as c,et as d,Ge as e,Ze as f,Je as g,ae as p,te as r,ot as s,_e as u,oe as v};
