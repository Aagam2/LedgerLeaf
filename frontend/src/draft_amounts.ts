import {scaled} from './numbers';

export type EnteredLine={quantityText:string;priceText:string;discountText:string};
// Mirrors the server's positive, half-up quantity × price calculation. These
// entered amounts exclude bill-level adjustments and any tax recalculation.
export function draftLineAmount(line:EnteredLine):number{
 const quantity=scaled(line.quantityText,3),price=scaled(line.priceText,2),discount=scaled(line.discountText,2);
 if(quantity<=0||quantity>1000000000)throw new Error('Enter a positive quantity within the supported limit.');
 if(price>100000000000)throw new Error('Unit price exceeds the supported limit.');
 const gross=(BigInt(quantity)*BigInt(price)+BigInt(500))/BigInt(1000);
 if(BigInt(discount)>gross)throw new Error('Discount cannot exceed the item amount.');
 const net=gross-BigInt(discount);
 if(net>BigInt(1000000000000))throw new Error('Line amount exceeds the supported limit.');
 return Number(net);
}
export function draftSubtotal(lines:EnteredLine[]):number{
 const total=lines.reduce((sum,line)=>sum+BigInt(draftLineAmount(line)),BigInt(0));
 if(total>BigInt(1000000000000))throw new Error('Item subtotal exceeds the supported limit.');
 return Number(total);
}
