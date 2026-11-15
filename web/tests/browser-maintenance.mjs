// Destructive acceptance runs only against the disposable CI database/browser.
import {chromium} from 'playwright';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
const base=process.env.STUDIO_URL||'http://127.0.0.1:8080';
const browser=await chromium.launch({headless:true,args:['--no-sandbox']});
const context=await browser.newContext({viewport:{width:1440,height:1000},acceptDownloads:true});
const page=await context.newPage(),errors=[];
page.on('pageerror',e=>errors.push(e.message));
const call=async(path,method='GET',body,headers={})=>{
 const r=await context.request.fetch(base+'/api'+path,{method,headers:{'Content-Type':'application/json',...headers},...(body===undefined?{}:{data:body})});
 assert.ok(r.ok(),`${path}: ${r.status()} ${await r.text()}`);return r.json();
};
const drafts=async(p=page)=>p.evaluate(()=>new Promise((resolve,reject)=>{const r=indexedDB.open('lce-studio-drafts',2);r.onsuccess=()=>{const tx=r.result.transaction('drafts'),req=tx.objectStore('drafts').getAll();tx.oncomplete=()=>{resolve(req.result);r.result.close()};tx.onerror=()=>reject(tx.error)}}));
async function open(){await page.getByRole('button',{name:'Backup & reset',exact:true}).click();await page.getByRole('dialog').waitFor();await page.getByRole('button',{name:'Create backup',exact:true}).waitFor();}
async function prepare(label='Create backup'){
 const event=page.waitForEvent('download');await page.getByRole('button',{name:label,exact:true}).click();const dl=await event;
 const b=JSON.parse(await fs.readFile(await dl.path(),'utf8'));
 await page.getByText('Backup retained on the server.',{exact:false}).waitFor();return b;
}
try {
 await fs.mkdir('test-results',{recursive:true});
 await page.goto(base);
 const p=await call('/example');p.id='maintenance-browser-project';p.name='Reset acceptance project';await call('/projects','POST',p);
 await page.getByRole('navigation',{name:'Studio planes and workspaces'}).getByRole('button',{name:'Components',exact:true}).click();
 await page.getByRole('button',{name:/^Component definitions/}).click();await page.getByRole('button',{name:'Configuration language',exact:true}).click();
 const studio=page.locator('.configuration-studio');
 await studio.getByRole('button',{name:'+ New component',exact:true}).click();
 await studio.getByRole('textbox',{name:'Component symbol',exact:true}).fill('BackupCounter');
 await studio.getByRole('button',{name:'Create component',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('.active-path')?.textContent==='components/BackupCounter/component.lce');
 await page.evaluate(()=>localStorage.setItem('studio.ribbon.backup-test','collapsed'));
 const old=await context.newPage();await old.goto(base);await old.getByRole('button',{name:'Configuration language',exact:true}).click();await old.locator('.monaco-editor').waitFor();
 console.log('Maintenance: both editor tabs opened.');
 const epoch=(await call('/studio/status')).epoch;
 await open();
 await page.route('**/api/studio/backup',route=>route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Injected backup failure'})}),{times:1});
 await page.getByRole('button',{name:'Create backup',exact:true}).click();await page.getByRole('alert').filter({hasText:'Injected backup failure'}).waitFor();
 assert.equal(await page.getByRole('button',{name:'Reset studio',exact:true}).isDisabled(),true);assert.ok((await call('/projects')).some(x=>x.id===p.id));
 await page.getByRole('button',{name:'Done',exact:true}).click();await page.getByRole('dialog').waitFor({state:'hidden'});
 await open();let b=await prepare();assert.ok(b.browser.drafts.some(d=>d.data.includes('BackupCounter')));assert.ok(b.database.includes('maintenance-browser-project'));
 await page.setViewportSize({width:390,height:844});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false);await page.screenshot({path:'test-results/maintenance-mobile.png',fullPage:true});await page.setViewportSize({width:1440,height:1000});
 // A server-side edit after backup must block reset, preserving the new edit.
 p.name='Changed after backup';await call('/projects','POST',p);
 await page.getByRole('textbox',{name:'Type RESET to confirm'}).fill('RESET');await page.getByRole('button',{name:'Reset studio',exact:true}).click();
 await page.getByRole('alert').filter({hasText:'studio changed'}).waitFor();assert.ok((await call('/projects')).some(x=>x.name===p.name));
 console.log('Maintenance: backup and stale snapshot check passed.');
 b=await prepare('Create fresh backup');
 await page.screenshot({path:'test-results/maintenance-backup-ready.png',fullPage:true});
 // Inject a browser storage failure AFTER the server commit. Recovery must
 // survive a reload and retry the same server operation without deleting twice.
 await page.evaluate(()=>{const original=IDBDatabase.prototype.transaction;let fail=true;IDBDatabase.prototype.transaction=function(names,mode,...rest){if(fail&&mode==='readwrite'&&Array.isArray(names)&&names.includes('drafts')&&names.includes('meta')){fail=false;throw new DOMException('Injected browser storage failure','QuotaExceededError')}return original.call(this,names,mode,...rest)}});
 await page.getByRole('textbox',{name:'Type RESET to confirm'}).fill('RESET');await page.getByRole('button',{name:'Reset studio',exact:true}).click();
 await page.getByRole('button',{name:'Resume recovery',exact:true}).waitFor();await page.getByRole('alert').filter({hasText:'Injected browser storage failure'}).waitFor();
 assert.deepEqual(await call('/projects'),[]);assert.ok((await drafts()).some(d=>d.data.includes('BackupCounter')),'pending browser copy retained');
 await page.reload();await page.getByRole('button',{name:'Resume recovery',exact:true}).waitFor();
 await Promise.all([page.waitForEvent('load'),page.getByRole('button',{name:'Resume recovery',exact:true}).click()]);
 await page.getByRole('button',{name:'Backup & reset',exact:true}).waitFor();
 assert.deepEqual(await call('/projects'),[]);assert.equal((await drafts()).some(d=>d.data.includes('BackupCounter')),false);
 assert.equal(await page.evaluate(()=>localStorage.getItem('studio.ribbon.backup-test')),null);
 const rejected=await context.request.post(base+'/api/projects',{headers:{'X-Studio-Epoch':epoch},data:p});assert.equal(rejected.status(),409);
 // An old editor must not resurrect its draft, even with normal autosave active.
 await old.locator('.monaco-editor').click();await old.keyboard.press('Control+End');await old.keyboard.insertText('\n// old tab must not save\n');
 await old.getByText('Studio was reset, restored, or paused in another tab.',{exact:false}).first().waitFor();
 assert.equal((await drafts()).some(d=>d.data.includes('old tab must not save')),false);
 await old.close();console.log('Maintenance: interrupted reset and stale tab checks passed.');
 await open();await page.getByRole('button',{name:'Restore',exact:true}).click();
 const tampered={...b,digest:'bad'};
 await page.getByLabel('Upload backup JSON').setInputFiles({name:'damaged.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(tampered))});
 await page.getByRole('alert').filter({hasText:'checksum mismatch'}).waitFor();assert.deepEqual(await call('/projects'),[]);
 await page.getByLabel('Upload backup JSON').setInputFiles({name:'studio.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(b))});
 await page.getByText('Validated ·',{exact:false}).waitFor();
 await prepare();await page.getByRole('textbox',{name:'Type RESTORE to confirm'}).fill('RESTORE');
 assert.equal(await page.getByRole('button',{name:'Close backup and reset'}).isVisible(),true);
 const closeBox=await page.getByRole('button',{name:'Close backup and reset'}).boundingBox();assert.ok(closeBox.y>=0&&closeBox.y+closeBox.height<=1000,'dialog header stays within viewport after scrolling to confirmation');
 const doneBox=await page.getByRole('button',{name:'Done',exact:true}).boundingBox();assert.ok(doneBox.y>=0&&doneBox.y+doneBox.height<=1000,'dialog footer stays within viewport');
 await page.screenshot({path:'test-results/maintenance-restore-ready.png',fullPage:true});
 await Promise.all([page.waitForEvent('load'),page.getByRole('button',{name:'Restore studio',exact:true}).click()]);
 await page.getByRole('button',{name:'Configuration language',exact:true}).click();await page.locator('.monaco-editor').waitFor();
 assert.ok((await call('/projects')).some(x=>x.id===p.id&&x.name===p.name));
 assert.ok((await drafts()).some(d=>d.data.includes('BackupCounter')));
 assert.equal(await page.evaluate(()=>localStorage.getItem('studio.ribbon.backup-test')),'collapsed');
 // Retained history is available after restoration, and backup-only cancellation
 // returns to an editable workspace without clearing records.
 await open();await page.getByRole('button',{name:'Restore',exact:true}).click();await page.getByLabel('Retained backups (latest 100)').selectOption(b.id);await page.getByRole('button',{name:'Use selected backup',exact:true}).click();await page.getByText('Validated ·',{exact:false}).waitFor();
 await page.getByRole('button',{name:'Done',exact:true}).click();await page.getByRole('dialog').waitFor({state:'hidden'});
 assert.ok((await call('/projects')).some(x=>x.id===p.id));assert.deepEqual(errors,[]);
 console.log('Maintenance browser acceptance passed: backup/download, mobile layout, stale-snapshot rejection, interrupted reset recovery, old-tab fencing, invalid-file rejection, file restore, retained history and cancel.');
} catch(e) {await page.screenshot({path:'test-results/maintenance-failure.png',fullPage:true}).catch(()=>{});console.error('Page errors',errors);throw e;} finally {await browser.close()}
