import {t} from './i18n';
import React,{useEffect,useState} from 'react';
import {Modal} from './modal';

// Successful saves advance resetKey; errors leave the form and its input intact.
export function ActionDialog({label,title,resetKey,busy,error,onOpen,children}:{label:string;title:string;resetKey:number|string;busy:boolean;error:string;onOpen:()=>void;children:React.ReactNode}){
 const[open,setOpen]=useState(false);
 useEffect(()=>{setOpen(false)},[resetKey]);
 return <><button type="button" className="primary" disabled={busy} onClick={()=>{onOpen();setOpen(true)}}>{t(label)}</button>{open&&<Modal title={title} busy={busy} onClose={()=>setOpen(false)}>{error&&<div className="error" role="alert">{error}</div>}{children}</Modal>}</>
}
