import {scaled} from './numbers';

export type PaymentPart={mode:string;amount:string};
// Tender is a cash-handling aid. Only allocated invoice amounts are posted.
export function paymentSummary(parts:PaymentPart[],total:number,tender:string){
 const lines=parts.map(p=>({mode:p.mode,amount:scaled(p.amount,2)}));
 const allocated=lines.reduce((n,p)=>n+p.amount,0);
 const cash=lines.filter(p=>p.mode==='CASH').reduce((n,p)=>n+p.amount,0);
 const received=cash===0?0:tender.trim()===''?cash:scaled(tender,2);
 return {lines,allocated,cash,remaining:total-allocated,received,change:received-cash};
}
