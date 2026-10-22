// Uses the server's PDF print model, never the editable form or a DOM screenshot.
export type PrintLine={Name:string;Quantity:string;Price:string;Discount:string;Amount:string;Tax:string};
export type PrintDocument={Title:string;Number:string;Business:string;Address:string;Party:string;PartyAddress:string;Date:string;DueDate:string;Notes:string;Total:string;Adjustments:string;TaxSummary:string;Logo?:string;LogoFormat?:string;Lines:PrintLine[]|null};
export type ImagePage={blob:Blob;filename:string};

export function wrapImageText(context:Pick<CanvasRenderingContext2D,'measureText'>,text:string,width:number):string[]{
 const output:string[]=[];
 for(const paragraph of String(text||'').replace(/\r\n?/g,'\n').split('\n')){
  let line='';
  for(const word of paragraph.split(/\s+/).filter(Boolean)){
   if(line&&context.measureText(line+' '+word).width<=width){line+=' '+word;continue}
   if(line){output.push(line);line=''}
   for(const character of Array.from(word)){
    if(line&&context.measureText(line+character).width>width){output.push(line);line=''}
    line+=character;
   }
  }
  output.push(line);
 }
 return output;
}
export async function renderInvoiceImages(documentData:PrintDocument,signal?:AbortSignal):Promise<ImagePage[]>{
 const canvas=document.createElement('canvas');canvas.width=1200;canvas.height=1697;
 const context=canvas.getContext('2d');if(!context)throw new Error('Image export is unavailable in this browser. Use Download PDF.');
 await document.fonts.ready;
 const pages:ImagePage[]=[],margin=80,width=1040,bottom=canvas.height-90;
 const name='Ledgerleaf-'+(documentData.Number||'document').replace(/[^a-z0-9._-]+/gi,'-').slice(0,80);
 let y=margin,page=0;
 const check=()=>{if(signal?.aborted)throw new DOMException('Export cancelled','AbortError')};
 const font=(size=20)=>{context.font=`${size}px "Segoe UI", Arial, sans-serif`;context.fillStyle='#18251e'};
 const start=()=>{check();page++;context.fillStyle='#fff';context.fillRect(0,0,canvas.width,canvas.height);y=margin; font();if(page>1){context.fillText(`${documentData.Title} · ${documentData.Number}`,margin,y);y+=48}};
 const finish=async()=>{
  check();font(16);context.fillStyle='#56645b';context.fillText(`Ledgerleaf · Page ${page}`,margin,canvas.height-42);
  const blob=await new Promise<Blob>((resolve,reject)=>canvas.toBlob(value=>value?resolve(value):reject(new Error('Image could not be created. Use Download PDF.')),'image/png'));
  check();pages.push({blob,filename:`${name}-page-${page}.png`});
 };
 const text=async(value:string,size=20,shade=false)=>{
  check();font(size);const lines=wrapImageText(context,value,width-24),height=Math.ceil(size*1.5);
  for(const line of lines){
   if(y+height>bottom){await finish();start();font(size)}
   if(shade){context.fillStyle='#eff3ef';context.fillRect(margin-8,y-4,width+16,height);context.fillStyle='#18251e'}
   context.fillText(line,margin+4,y+size);y+=height;
  }
 };
 try{
  start();
  if(documentData.Logo){
   const format=(documentData.LogoFormat||'').toLowerCase();
   if(!['png','jpeg','jpg'].includes(format))throw new Error('The saved logo cannot be exported. Use Download PDF.');
   const logo=new Image();logo.src=`data:image/${format==='jpg'?'jpeg':format};base64,${documentData.Logo}`;await logo.decode();check();
   const scale=Math.min(200/logo.naturalWidth,120/logo.naturalHeight);context.drawImage(logo,margin,y,logo.naturalWidth*scale,logo.naturalHeight*scale);y+=logo.naturalHeight*scale+20;
  }
  await text(documentData.Business,32);await text(documentData.Address);y+=16;
  await text(`${documentData.Title} · ${documentData.Number}`,25);
  await text(`Date: ${documentData.Date}${documentData.DueDate?'   Due: '+documentData.DueDate:''}`);y+=14;
  await text(`Party: ${documentData.Party}`);await text(documentData.PartyAddress);y+=18;
  for(const line of documentData.Lines||[]){
   const quantity=`Quantity: ${line.Quantity}   |   Unit price: INR ${line.Price}`;
   const amount=`Discount: INR ${line.Discount}   |   Amount: INR ${line.Amount}`;
   const measure=(value:string,size:number)=>{font(size);return wrapImageText(context,value,width-24).length*Math.ceil(size*1.5)};
   const itemHeight=measure(line.Name,22)+measure(quantity,20)+measure(amount,20)+(line.Tax?measure(line.Tax,18):0)+16;
   // Keep ordinary items together; exceptionally long items can span pages.
   if(itemHeight<=bottom-margin-48&&y+itemHeight>bottom){await finish();start()}
   await text(line.Name,22,true);await text(quantity);await text(amount);
   if(line.Tax)await text(line.Tax,18);y+=16;
  }
  if(documentData.TaxSummary)await text(documentData.TaxSummary);
  if(documentData.Adjustments)await text(documentData.Adjustments);
  y+=12;await text(`Total: ${/^-?\d+(?:\.\d+)?$/.test(documentData.Total)?'INR '+documentData.Total:documentData.Total}`,28);y+=16;
  if(documentData.Notes)await text(documentData.Notes,18);
  await finish();
  if(pages.length===1)pages[0].filename=`${name}.png`;
  return pages;
 }finally{canvas.width=0;canvas.height=0}
}
