import {t} from './i18n';
import React,{useEffect,useId,useRef} from 'react';

// Native dialogs provide focus trapping, Escape handling and an inert background.
export function Modal({title,onClose,busy=false,confirmDiscard=true,children}:{title:string;onClose:()=>void;busy?:boolean;confirmDiscard?:boolean;children:React.ReactNode}){
 const ref=useRef<HTMLDialogElement>(null),dirty=useRef(false),titleID=useId();
 const close=()=>{if(!busy&&(!confirmDiscard||!dirty.current||confirm(t('Discard unsaved changes?'))))onClose()};
 useEffect(()=>{const dialog=ref.current!;const previous=document.activeElement as HTMLElement|null;dialog.showModal();const overflow=document.body.style.overflow;document.body.style.overflow='hidden';return()=>{dialog.close();document.body.style.overflow=overflow;previous?.focus()}},[]);
 return <dialog ref={ref} className="modal" aria-labelledby={titleID} onCancel={e=>{e.preventDefault();e.stopPropagation();close()}} onChange={()=>{dirty.current=true}}><div className="modal-heading"><h2 id={titleID}>{t(title)}</h2><button type="button" aria-label={`Close ${title}`} disabled={busy} onClick={close}>✕</button></div><div className="modal-body">{children}</div></dialog>
}
