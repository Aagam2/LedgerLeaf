import React,{useEffect,useRef,useState} from 'react';
import {api} from './api';
import {t} from './i18n';
import {renderInvoiceImages} from './invoice_image';

type Download={url:string;filename:string};
export function DocumentDownloads({pdfURL}:{pdfURL:string}){
 const[busy,setBusy]=useState(false),[error,setError]=useState(''),[downloads,setDownloads]=useState<Download[]>([]);
 const controller=useRef<AbortController|null>(null),urls=useRef<string[]>([]);
 useEffect(()=>{setDownloads([]);setError('');setBusy(false);return()=>{controller.current?.abort();urls.current.forEach(url=>URL.revokeObjectURL(url));urls.current=[]}},[pdfURL]);
 async function images(){
  if(busy)return;
  const pending=new AbortController();controller.current=pending;setBusy(true);setError('');
  urls.current.forEach(url=>URL.revokeObjectURL(url));urls.current=[];setDownloads([]);
  try{
   const[path,query='']=pdfURL.split('?');const params=new URLSearchParams(query);params.set('format','image-data');
   const data=await api(path.replace(/^\/api\//,'')+'?'+params.toString());
   if(pending.signal.aborted)return;
   const pages=await renderInvoiceImages(data,pending.signal);
   if(pending.signal.aborted)return;
   const files=pages.map(page=>({url:URL.createObjectURL(page.blob),filename:page.filename}));urls.current=files.map(file=>file.url);setDownloads(files);
   if(files.length===1){const link=document.createElement('a');link.href=files[0].url;link.download=files[0].filename;link.click()}
  }catch(e){if(!pending.signal.aborted)setError(String(e))}finally{if(!pending.signal.aborted)setBusy(false)}
 }
 return <div className="document-downloads"><div className="actions"><a className="download" href={pdfURL} target="_blank" rel="noreferrer">{t('Download PDF')}</a><button type="button" disabled={busy} onClick={images}>{t(busy?'Creating images…':'Export image (PNG)')}</button></div>{error&&<p className="error" role="alert">{error}</p>}{downloads.length>0&&<div className="image-downloads"><p role="status">{t(downloads.length===1?'Image ready.':'Images ready. Download each page below.')} {downloads.length>1&&`${downloads.length} ${t('pages')}`}</p>{downloads.map((file,index)=><a key={file.url} href={file.url} download={file.filename}>{t('Download PNG')} {downloads.length>1?`${t('Page')} ${index+1}`:''}</a>)}</div>}</div>;
}
