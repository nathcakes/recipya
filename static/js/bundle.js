(()=>{var U=Object.create;var F=Object.defineProperty;var G=Object.getOwnPropertyDescriptor;var J=Object.getOwnPropertyNames;var Q=Object.getPrototypeOf,X=Object.prototype.hasOwnProperty;var Y=n=>F(n,"__esModule",{value:!0});var V=(n,o)=>()=>(o||n((o={exports:{}}).exports,o),o.exports);var Z=(n,o,p)=>{if(o&&typeof o=="object"||typeof o=="function")for(let m of J(o))!X.call(n,m)&&m!=="default"&&F(n,m,{get:()=>o[m],enumerable:!(p=G(o,m))||p.enumerable});return n},j=n=>Z(Y(F(n!=null?U(Q(n)):{},"default",n&&n.__esModule&&"default"in n?{get:()=>n.default,enumerable:!0}:{value:n,enumerable:!0})),n);var W=V((H,I)=>{(function(n,o){"use strict";typeof define=="function"&&define.amd?define([],o):typeof I=="object"&&I.exports?I.exports=o():n.picoModal=o()})(H,function(){"use strict";function n(e){return typeof Node=="object"?e instanceof Node:e&&typeof e=="object"&&typeof e.nodeType=="number"}function o(e){return typeof e=="string"}function p(){var e=[];return{watch:e.push.bind(e),trigger:function(i,r){for(var h=!0,k={detail:r,preventDefault:function(){h=!1}},w=0;w<e.length;w++)e[w](i,k);return h}}}function m(e){return window.getComputedStyle(e).display==="none"}function v(e){this.elem=e}v.make=function(e,i){typeof e=="string"&&(e=document.querySelector(e));var r=document.createElement(i||"div");return(e||document.body).appendChild(r),new v(r)},v.prototype={child:function(e){return v.make(this.elem,e)},stylize:function(e){e=e||{},typeof e.opacity!="undefined"&&(e.filter="alpha(opacity="+e.opacity*100+")");for(var i in e)e.hasOwnProperty(i)&&(this.elem.style[i]=e[i]);return this},clazz:function(e){return this.elem.className+=" "+e,this},html:function(e){return n(e)?this.elem.appendChild(e):this.elem.innerHTML=e,this},onClick:function(e){return this.elem.addEventListener("click",e),this},destroy:function(){this.elem.parentNode.removeChild(this.elem)},hide:function(){this.elem.style.display="none"},show:function(){this.elem.style.display="block"},attr:function(e,i){return i!==void 0&&this.elem.setAttribute(e,i),this},anyAncestor:function(e){for(var i=this.elem;i;){if(e(new v(i)))return!0;i=i.parentNode}return!1},isVisible:function(){return!m(this.elem)}};function t(e,i){return v.make(e("parent")).clazz("pico-overlay").clazz(e("overlayClass","")).stylize({display:"none",position:"fixed",top:"0px",left:"0px",height:"100%",width:"100%",zIndex:1e4}).stylize(e("overlayStyles",{opacity:.5,background:"#000"})).onClick(function(){e("overlayClose",!0)&&i()})}var c=1;function E(e,i){var r=e("width","auto");typeof r=="number"&&(r=""+r+"px");var h=e("modalId","pico-"+c++),k=v.make(e("parent")).clazz("pico-content").clazz(e("modalClass","")).stylize({display:"none",position:"fixed",zIndex:10001,left:"50%",top:"38.1966%",maxHeight:"90%",boxSizing:"border-box",width:r,"-ms-transform":"translate(-50%,-38.1966%)","-moz-transform":"translate(-50%,-38.1966%)","-webkit-transform":"translate(-50%,-38.1966%)","-o-transform":"translate(-50%,-38.1966%)",transform:"translate(-50%,-38.1966%)"}).stylize(e("modalStyles",{overflow:"auto",backgroundColor:"white",padding:"20px",borderRadius:"5px"})).html(e("content")).attr("id",h).attr("role","dialog").attr("aria-labelledby",e("ariaLabelledBy")).attr("aria-describedby",e("ariaDescribedBy",h)).onClick(function(w){var g=new v(w.target).anyAncestor(function(s){return/\bpico-close\b/.test(s.elem.className)});g&&i()});return k}function f(e,i){if(i("closeButton",!0))return e.child("button").html(i("closeHtml","&#xD7;")).clazz("pico-close").clazz(i("closeClass","")).stylize(i("closeStyles",{borderRadius:"2px",border:0,padding:0,cursor:"pointer",height:"15px",width:"15px",position:"absolute",top:"5px",right:"5px",fontSize:"16px",textAlign:"center",lineHeight:"15px",background:"#CCC"})).attr("aria-label",i("close-label","Close"))}function l(e){return function(){return e().elem}}var b=p(),C=p();document.documentElement.addEventListener("keydown",function(i){var r=i.which||i.keyCode;r===27?b.trigger():r===9&&C.trigger(i)});function x(e,i){function r(s,a){var d=s.msMatchesSelector||s.webkitMatchesSelector||s.matches;return d.call(s,a)}function h(s){return m(s)||r(s,":disabled")||s.hasAttribute("contenteditable")?!1:s.hasAttribute("tabindex")||r(s,"input,select,textarea,button,a[href],area[href],iframe")}function k(s){for(var a=s.getElementsByTagName("*"),d=0;d<a.length;d++)if(h(a[d]))return a[d]}function w(s){for(var a=s.getElementsByTagName("*"),d=a.length;d--;)if(h(a[d]))return a[d]}var g;e.beforeShow(function(){g=document.activeElement}),e.afterShow(function(){if(i()){var a=k(e.modalElem());a&&a.focus()}}),e.afterClose(function(){i()&&g&&g.focus(),g=null}),C.watch(function(a){if(i()&&e.isVisible()){var d=k(e.modalElem()),T=w(e.modalElem()),y=a.shiftKey?d:T;y===document.activeElement&&((a.shiftKey?T:d).focus(),a.preventDefault())}})}function M(e,i){var r,h=new v(document.body);e.beforeShow(function(){r=h.elem.style.overflow,i()&&h.stylize({overflow:"hidden"})}),e.afterClose(function(){h.stylize({overflow:r})})}return function(i){(o(i)||n(i))&&(i={content:i});var r=p(),h=p(),k=p(),w=p(),g=p();function s(u,L){var z=i[u];return typeof z=="function"&&(z=z(L)),z===void 0?L:z}var a=B.bind(window,"modal"),d=B.bind(window,"overlay"),T=B.bind(window,"close"),y;function P(u){d().hide(),a().hide(),g.trigger(y,u)}function A(u){w.trigger(y,u)&&P(u)}function N(u){return function(){return u.apply(this,arguments),y}}var S;function B(u,L){if(!S){var z=E(s,A);S={modal:z,overlay:t(s,A),close:f(z,s)},r.trigger(y,L)}return S[u]}return y={modalElem:l(a),closeElem:l(T),overlayElem:l(d),buildDom:N(B.bind(null,null)),isVisible:function(){return!!(S&&a&&a().isVisible())},show:function(u){return h.trigger(y,u)&&(d().show(),T(),a().show(),k.trigger(y,u)),this},close:N(A),forceClose:N(P),destroy:function(){a().destroy(),d().destroy(),d=a=T=void 0},options:function(u){Object.keys(u).map(function(L){i[L]=u[L]})},afterCreate:N(r.watch),beforeShow:N(h.watch),afterShow:N(k.watch),beforeClose:N(w.watch),afterClose:N(g.watch)},x(y,s.bind(null,"focus",!0)),M(y,s.bind(null,"bodyOverflow",!0)),b.watch(function(){s("escCloses",!0)&&y.isVisible()&&y.close()}),y}})});var K=V((D,_)=>{(function(n,o){typeof _=="object"&&_.exports?_.exports=o():n.Toastify=o()})(D,function(n){var o=function(t){return new o.lib.init(t)},p="1.11.2";o.defaults={oldestFirst:!0,text:"Toastify is awesome!",node:void 0,duration:3e3,selector:void 0,callback:function(){},destination:void 0,newWindow:!1,close:!1,gravity:"toastify-top",positionLeft:!1,position:"",backgroundColor:"",avatar:"",className:"",stopOnFocus:!0,onClick:function(){},offset:{x:0,y:0},escapeMarkup:!0,style:{background:""}},o.lib=o.prototype={toastify:p,constructor:o,init:function(t){return t||(t={}),this.options={},this.toastElement=null,this.options.text=t.text||o.defaults.text,this.options.node=t.node||o.defaults.node,this.options.duration=t.duration===0?0:t.duration||o.defaults.duration,this.options.selector=t.selector||o.defaults.selector,this.options.callback=t.callback||o.defaults.callback,this.options.destination=t.destination||o.defaults.destination,this.options.newWindow=t.newWindow||o.defaults.newWindow,this.options.close=t.close||o.defaults.close,this.options.gravity=t.gravity==="bottom"?"toastify-bottom":o.defaults.gravity,this.options.positionLeft=t.positionLeft||o.defaults.positionLeft,this.options.position=t.position||o.defaults.position,this.options.backgroundColor=t.backgroundColor||o.defaults.backgroundColor,this.options.avatar=t.avatar||o.defaults.avatar,this.options.className=t.className||o.defaults.className,this.options.stopOnFocus=t.stopOnFocus===void 0?o.defaults.stopOnFocus:t.stopOnFocus,this.options.onClick=t.onClick||o.defaults.onClick,this.options.offset=t.offset||o.defaults.offset,this.options.escapeMarkup=t.escapeMarkup!==void 0?t.escapeMarkup:o.defaults.escapeMarkup,this.options.style=t.style||o.defaults.style,t.backgroundColor&&(this.options.style.background=t.backgroundColor),this},buildToast:function(){if(!this.options)throw"Toastify is not initialized";var t=document.createElement("div");t.className="toastify on "+this.options.className,this.options.position?t.className+=" toastify-"+this.options.position:this.options.positionLeft===!0?(t.className+=" toastify-left",console.warn("Property `positionLeft` will be depreciated in further versions. Please use `position` instead.")):t.className+=" toastify-right",t.className+=" "+this.options.gravity,this.options.backgroundColor&&console.warn('DEPRECATION NOTICE: "backgroundColor" is being deprecated. Please use the "style.background" property.');for(var c in this.options.style)t.style[c]=this.options.style[c];if(this.options.node&&this.options.node.nodeType===Node.ELEMENT_NODE)t.appendChild(this.options.node);else if(this.options.escapeMarkup?t.innerText=this.options.text:t.innerHTML=this.options.text,this.options.avatar!==""){var E=document.createElement("img");E.src=this.options.avatar,E.className="toastify-avatar",this.options.position=="left"||this.options.positionLeft===!0?t.appendChild(E):t.insertAdjacentElement("afterbegin",E)}if(this.options.close===!0){var f=document.createElement("span");f.innerHTML="&#10006;",f.className="toast-close",f.addEventListener("click",function(i){i.stopPropagation(),this.removeElement(this.toastElement),window.clearTimeout(this.toastElement.timeOutValue)}.bind(this));var l=window.innerWidth>0?window.innerWidth:screen.width;(this.options.position=="left"||this.options.positionLeft===!0)&&l>360?t.insertAdjacentElement("afterbegin",f):t.appendChild(f)}if(this.options.stopOnFocus&&this.options.duration>0){var b=this;t.addEventListener("mouseover",function(i){window.clearTimeout(t.timeOutValue)}),t.addEventListener("mouseleave",function(){t.timeOutValue=window.setTimeout(function(){b.removeElement(t)},b.options.duration)})}if(typeof this.options.destination!="undefined"&&t.addEventListener("click",function(i){i.stopPropagation(),this.options.newWindow===!0?window.open(this.options.destination,"_blank"):window.location=this.options.destination}.bind(this)),typeof this.options.onClick=="function"&&typeof this.options.destination=="undefined"&&t.addEventListener("click",function(i){i.stopPropagation(),this.options.onClick()}.bind(this)),typeof this.options.offset=="object"){var C=m("x",this.options),x=m("y",this.options),M=this.options.position=="left"?C:"-"+C,e=this.options.gravity=="toastify-top"?x:"-"+x;t.style.transform="translate("+M+","+e+")"}return t},showToast:function(){this.toastElement=this.buildToast();var t;if(typeof this.options.selector=="string"?t=document.getElementById(this.options.selector):this.options.selector instanceof HTMLElement||typeof ShadowRoot!="undefined"&&this.options.selector instanceof ShadowRoot?t=this.options.selector:t=document.body,!t)throw"Root element is not defined";var c=o.defaults.oldestFirst?t.firstChild:t.lastChild;return t.insertBefore(this.toastElement,c),o.reposition(),this.options.duration>0&&(this.toastElement.timeOutValue=window.setTimeout(function(){this.removeElement(this.toastElement)}.bind(this),this.options.duration)),this},hideToast:function(){this.toastElement.timeOutValue&&clearTimeout(this.toastElement.timeOutValue),this.removeElement(this.toastElement)},removeElement:function(t){t.className=t.className.replace(" on",""),window.setTimeout(function(){this.options.node&&this.options.node.parentNode&&this.options.node.parentNode.removeChild(this.options.node),t.parentNode&&t.parentNode.removeChild(t),this.options.callback.call(t),o.reposition()}.bind(this),400)}},o.reposition=function(){for(var t={top:15,bottom:15},c={top:15,bottom:15},E={top:15,bottom:15},f=document.getElementsByClassName("toastify"),l,b=0;b<f.length;b++){v(f[b],"toastify-top")===!0?l="toastify-top":l="toastify-bottom";var C=f[b].offsetHeight;l=l.substr(9,l.length-1);var x=15,M=window.innerWidth>0?window.innerWidth:screen.width;M<=360?(f[b].style[l]=E[l]+"px",E[l]+=C+x):v(f[b],"toastify-left")===!0?(f[b].style[l]=t[l]+"px",t[l]+=C+x):(f[b].style[l]=c[l]+"px",c[l]+=C+x)}return this};function m(t,c){return c.offset[t]?isNaN(c.offset[t])?c.offset[t]:c.offset[t]+"px":"0px"}function v(t,c){return!t||typeof c!="string"?!1:!!(t.className&&t.className.trim().split(/\s+/gi).indexOf(c)>-1)}return o.lib.init.prototype=o.lib,o})});var R=j(W()),q=j(K());window.openModal=n=>{for(n=n.target;!n.id;)n=n.parentElement;window.modal=(0,R.default)({content:document.getElementById(n.id.split("-open-button")[0])}).beforeShow(o=>o.modalElem().children[0].classList.remove("hidden")).beforeClose(o=>o.modalElem().children[0].classList.add("hidden")).show()};window.showSuccessToast=n=>(0,q.default)({text:n,close:!0,position:"center",style:{background:"#27ae60"}}).showToast();var O={close:"M6 18L18 6M6 6l12 12",hamburger:"M4 6h16M4 12h16M4 18h16"};document.getElementById("nav__avatar-button").addEventListener("mousedown",()=>{document.getElementById("nav__avatar-content").classList.toggle("hidden")});document.onclick=({target:n})=>{let o=n.id,p=document.getElementById("nav__avatar-button"),m=document.getElementById("nav__avatar-content");[m.id,p.id].includes(o)||m.classList.add("hidden")};document.getElementById("nav__menu-icon").addEventListener("mousedown",()=>{let n=document.getElementById("nav__menu-icon-path");n.getAttribute("d")===O.close?n.setAttribute("d",O.hamburger):n.setAttribute("d",O.close),document.getElementById("nav__sidebar").classList.toggle("hidden")});var $=document.getElementById("sidebar__recipes");switch(document.location.pathname){case"/":$.classList.add("border-l-4","border-red-600");break;default:break}})();
/*!
 * Toastify js 1.11.2
 * https://github.com/apvarun/toastify-js
 * @license MIT licensed
 *
 * Copyright (C) 2018 Varun A P
 */
