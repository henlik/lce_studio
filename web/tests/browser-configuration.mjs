// Real Monaco and production CSS, served by the Go application under its CSP.
// Run after `npm run build`, starting the Go API first. CI installs Chromium.
import {chromium} from 'playwright';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
const base=process.env.STUDIO_URL||'http://127.0.0.1:8080';
const browser=await chromium.launch({headless:true,...(process.env.CHROMIUM_PATH?{executablePath:process.env.CHROMIUM_PATH}:{}),args:['--no-sandbox']});
const page=await browser.newPage({viewport:{width:1440,height:1000}}),errors=[];
page.on('pageerror',e=>errors.push(e.message));page.on('console',m=>{if(m.type()==='error'&&!m.text().includes('favicon'))errors.push(m.text())});
await fs.mkdir('test-results',{recursive:true});
try {
 await page.goto(base);await page.getByRole('navigation',{name:'Studio planes and workspaces'}).getByRole('button',{name:'Components',exact:true}).click();
 await page.getByRole('button',{name:/^Component definitions/}).click();await page.getByRole('button',{name:'Configuration language',exact:true}).click();
 const studio=page.locator('.configuration-studio');await studio.getByRole('button',{name:'Nested component example',exact:true}).click();
 await page.locator('.monaco-editor').waitFor();await studio.getByRole('button',{name:'Reception composition',exact:true}).first().click();
 await page.waitForFunction(()=>document.querySelector('.monaco-editor .view-lines')?.textContent?.includes('Reception'));
 await studio.getByRole('button',{name:'Queue management',exact:true}).first().click();
 await studio.getByRole('button',{name:'Ticket selection',exact:true}).first().click();
 assert.ok((await studio.locator('.config-breadcrumbs').textContent()).includes('Queue management'));
 await studio.getByRole('button',{name:'Back',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('.active-path')?.textContent?.includes('queue/component.lce'));
 await studio.getByRole('button',{name:'Forward',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('.active-path')?.textContent?.includes('ticket/component.lce'));
 // Real keyboard editing, per-file undo and draft persistence.
 const editor=page.locator('.monaco-editor');await editor.click();await page.keyboard.press('Control+End');await page.keyboard.type('\n-- browser draft marker');
 const downloadReady=page.waitForEvent('download');await studio.getByRole('button',{name:'Export source',exact:true}).click();const download=await downloadReady;const exported=JSON.parse(await fs.readFile(await download.path(),'utf8'));assert.ok(exported.files['reception/components/queue/components/ticket/component.lce'].includes('-- browser draft marker'),'typed spaces and comment must be retained in canonical source');
 await studio.getByRole('button',{name:'Queue management',exact:true}).first().click();await studio.getByRole('button',{name:'Ticket selection',exact:true}).first().click();
 await page.waitForFunction(()=>document.querySelector('.monaco-editor .view-lines')?.textContent?.replace(/\u00a0/g,' ').includes('browser draft marker'));
 await editor.click();await page.keyboard.press('Control+z');
 await page.waitForFunction(()=>!document.querySelector('.monaco-editor .view-lines')?.textContent?.replace(/\u00a0/g,' ').includes('browser draft marker'));
 await page.keyboard.press('Control+f');await page.locator('.monaco-editor .find-widget.visible').waitFor();await page.keyboard.press('Escape');
 await studio.getByRole('button',{name:'Testbench',exact:true}).click();await studio.getByRole('button',{name:'Run component tests',exact:true}).click();
 await page.waitForFunction(()=>document.querySelector('.configuration-main .config-success')?.textContent?.includes('1 experiments'));
 await studio.getByRole('button',{name:'Run all tests',exact:true}).first().click();
 await page.waitForFunction(()=>document.querySelector('.configuration-main .config-success')?.textContent?.includes('4 experiments'));
 await studio.getByRole('button',{name:'Source',exact:true}).first().click();
 assert.equal(await editor.evaluate(el=>getComputedStyle(el).backgroundColor),'rgb(24, 35, 47)');
 assert.ok(await page.evaluate(()=>document.fonts.check('14px codicon')));
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,'desktop horizontal overflow');
 await page.waitForFunction(()=>document.querySelectorAll('.studio-toast').length===0);
 await page.screenshot({path:'test-results/configuration-desktop.png',fullPage:true});
 await page.setViewportSize({width:390,height:844});await page.waitForTimeout(250);
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth+1),false,'mobile horizontal overflow');
 assert.equal(await page.evaluate(()=>{const rail=document.querySelector('.app-rail').getBoundingClientRect();return [...document.querySelectorAll('.studio-navigation .plane-toggle')].every(el=>{const r=el.getBoundingClientRect();return r.top>=rail.top&&r.bottom<=rail.bottom})}),true,'collapsed mobile plane navigation stays within the header');
 await page.screenshot({path:'test-results/configuration-mobile.png',fullPage:true});
 assert.deepEqual(errors,[],'browser/CSP errors');console.log('Browser acceptance passed: Monaco, nested navigation, per-file undo, find, scoped tests, production CSS and responsive widths.');
}catch(error){console.error('Browser errors:',errors);console.error('Editor text:',await page.locator('.monaco-editor .view-lines').textContent().catch(()=>''));await page.screenshot({path:'test-results/configuration-failure.png',fullPage:true}).catch(()=>{});throw error}finally{await browser.close()}
