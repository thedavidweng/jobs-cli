function(artifact) {
 const visible=e=>!!e.getClientRects().length && !e.closest('[hidden],[aria-hidden="true"]');
 const candidate=artifact.candidate,answers=Object.fromEntries((artifact.answers||[]).map(a=>[a.question_id,a.value]));
 const names={firstName:'first_name',lastName:'last_name',email:'email',emailAddress:'email',phoneNumber:'phone',addressLine1:'address_line',city:'location',postalCode:'postal_code',country:'country',countryRegion:'region'};
 const fields={...candidate,...candidate.address};
 const set=(e,value)=>{
  if (value===undefined || value===null || value==='') return '';
  if (e.tagName==='SELECT') {
   const matches=[...e.options].filter(o=>o.value===String(value) || o.textContent.trim()===String(value));
   if (matches.length!==1) return 'Unsupported option for '+(e.name||e.id)+': '+value;
   value=matches[0].value;
  }
  if (e.type==='checkbox') {if(typeof value!=='boolean')return 'Explicit boolean required for '+(e.name||e.id);if(e.checked!==value)e.click();return '';}
  if (e.type==='radio') {if(String(value)===e.value && !e.checked)e.click();return '';}
  if (e.value===String(value)) return '';
  const prototype=e.tagName==='SELECT'?HTMLSelectElement.prototype:e.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype;
  Object.getOwnPropertyDescriptor(prototype,'value').set.call(e,String(value));
  e.dispatchEvent(new Event('input',{bubbles:true}));e.dispatchEvent(new Event('change',{bubbles:true}));
  return '';
 };
 const sectionIDs={work:'workExperienceSection',education:'educationSection',skills:'skillsSection',languages:'languagesSection',certificates:'certificationsSection'};
 const rowIDs={work:'workExperience',education:'education',skills:'skill',languages:'language',certificates:'certification'};
 const keys={company:'name',jobTitle:'position',startDate:'startDate',endDate:'endDate',description:'summary',school:'institution',degree:'studyType',fieldOfStudy:'area',skill:'name',language:'language',fluency:'fluency',certification:'name'};
 // Reconcile by explicit employer+title or institution+degree identity. Never delete rows.
 for (const [name,id] of Object.entries(sectionIDs)) {
  const section=document.querySelector('[data-automation-id='+JSON.stringify(id)+']');
  const entries=candidate[name]||[];
  if (!section || !visible(section)) continue;
  for(const entry of entries) {
   const rows=[...section.querySelectorAll('[data-automation-id='+JSON.stringify(rowIDs[name])+']')];
   const identity=name==='work'?['company','jobTitle']:name==='education'?['school','degree']:[rowIDs[name]];
   const matches=rows.filter(row=>identity.every(key=>{
    const control=row.querySelector('[data-automation-id='+JSON.stringify(key)+']');
    const expected=String(entry[keys[key]]||'');
    return control && (control.value===expected || control.tagName==='SELECT' && control.selectedOptions[0]?.textContent.trim()===expected);
   }));
   if(matches.length>1)return 'Ambiguous existing '+name+' rows; reconcile manually';
   let row=matches[0];
   if(!row) {
    const empty=rows.filter(row=>[...row.querySelectorAll('input,textarea,select')].every(e=>!e.value));
    if(empty.length>1)return 'Ambiguous empty '+name+' rows';
    row=empty[0];
    if(!row) {
     const add=[...section.querySelectorAll('button[data-automation-id="addButton"]')].filter(visible);
     if(add.length!==1)return 'Unsupported '+name+' add-row control';
     add[0].click();return 'row_added:'+name+':'+rows.length;
    }
   }
   for(const e of row.querySelectorAll('input,textarea,select')) {
    const value=entry[keys[e.getAttribute('data-automation-id')]];
    if(e.type==='date' && value && !/^\d{4}-\d{2}-\d{2}$/.test(value))return 'Exact date required for '+name+': '+value;
    if(e.required && !value)return 'Missing '+name+' '+(e.name||e.getAttribute('data-automation-id'));
    const pending=set(e,value);if(pending)return pending;
   }
  }
 }
 for(const e of document.querySelectorAll('input,textarea,select')) {
  if(!visible(e) || ['file','hidden','password'].includes(e.type) || e.closest('[data-automation-id$="Section"]'))continue;
  const name=names[e.getAttribute('data-automation-id')];
  const value=name?fields[name]:answers[e.name||e.id||e.getAttribute('data-automation-id')];
  const pending=set(e,value);if(pending)return pending;
 }
 return '';
}
