import React,{useEffect,useState} from 'react';
import {t} from './i18n';
import {Role,visiblePage} from './permissions';

export const pageLabel=(page:string)=>page==='Commercial documents'?'Orders & quotations':page;

export const navigationGroups = [
 {title:'Everyday',pages:['Overview','Billing','Commercial documents','POS','Parties','Products','Expenses','Reports']},
 {title:'Other documents',pages:['Goods receipts','Dispatch challans','Credit notes']},
 {title:'Stock',pages:['Inventory','Stock history','Stock changes','Warehouses']},
 {title:'Accounting',pages:['Party balances','Journal entries','Banks & periods','Trial balance','Journals']},
 {title:'Settings',pages:['Businesses','GST configuration','Catalog settings','Unit conversions','Price lists','Import & export','Backup & restore','Users & permissions']},
];
function NavigationGroup({title,active,children}:{title:string;active:boolean;children:React.ReactNode}){
 const[open,setOpen]=useState(active);
 useEffect(()=>{if(active)setOpen(true)},[active]);
 return <details className="navigation-group" open={open} onToggle={e=>setOpen(e.currentTarget.open)}>
  <summary className={active?'has-current-page':''}>{t(title)}</summary>
  <div className="navigation-links">{children}</div>
 </details>;
}
export function Navigation({role,hasBusiness,page,onNavigate}:{role:Role;hasBusiness:boolean;page:string;onNavigate:(page:string)=>void}){
 const link=(destination:string)=><button key={destination} type="button" aria-current={destination===page?'page':undefined} className={destination===page?'active':''} onClick={()=>onNavigate(destination)}>{t(pageLabel(destination))}</button>;
 return <nav className="app-navigation" aria-label={t('Main menu')}>
  {navigationGroups.map(group=>{
   const pages=group.pages.filter(destination=>visiblePage(role,destination)&&(hasBusiness||['Overview','Businesses','Users & permissions'].includes(destination)));
   if(!pages.length)return null;
   return group.title==='Everyday'?<div key={group.title} className="navigation-everyday"><div className="navigation-caption">{t(group.title)}</div><div className="navigation-links">{pages.map(link)}</div></div>:
    <NavigationGroup key={group.title} title={group.title} active={pages.includes(page)}>{pages.map(link)}</NavigationGroup>;
  })}
  <div className="navigation-help">{link('Help')}</div>
 </nav>;
}
