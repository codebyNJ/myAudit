// Regression check for #20: the ticket drawer must not move the line the
// reader is looking at when content above it changes height.
//
// Runs in WebKit on purpose. The desktop app is a Tauri WKWebView, and WebKit
// does not implement CSS scroll anchoring — so this bug is invisible in Chrome
// and only shows up where users actually hit it. Chromium is checked too, to
// prove the hook stays out of the way where the browser handles it natively.
//
//   make dev   # in another shell
//   node scripts/drawer-anchor-smoke.mjs
import { chromium, webkit } from 'playwright'

const RUN = process.env.RUN_ID || '9b0fe6e7-ec6f-4a9f-a32e-b879750511f3'
const BASE = process.env.BASE || 'http://localhost:7788'
const GROWTH = 300

const measure = async () => {
  const frame = () => new Promise((r) => requestAnimationFrame(r))
  const el = document.querySelector('.drawer-body')
  if (!el) throw new Error('drawer did not open')

  // Returns how far the reader's line moved. 0 is the only acceptable answer.
  const probe = async (scrollTo, mutate) => {
    const spacer = document.createElement('div')
    spacer.style.cssText = 'height:40px;flex:none'
    el.insertBefore(spacer, el.firstElementChild)
    await frame()

    el.scrollTop = scrollTo(el)
    await frame()
    const mark = [...el.children].find((c) => c.offsetTop > el.scrollTop) || el.lastElementChild
    const before = Math.round(mark.getBoundingClientRect().top)

    mutate(spacer)
    await frame(); await frame(); await frame()

    const moved = Math.round(mark.getBoundingClientRect().top) - before
    spacer.remove()
    await frame()
    return moved
  }

  const mid = (e) => Math.round(e.scrollHeight * 0.55)
  const grow = (sp) => { sp.style.height = `${40 + 300}px` }

  return {
    // A QA preview image decoding above the viewport.
    imageDecode: await probe(mid, grow),
    // The fix-diff block arriving when a ticket changes status mid-audit.
    insertion: await probe(mid, (sp) => {
      const tall = document.createElement('div')
      tall.style.cssText = 'height:300px;flex:none'
      sp.parentElement.insertBefore(tall, sp.nextSibling)
      setTimeout(() => tall.remove(), 400)
    }),
    // Reading from the top: nothing above, so nothing should move.
    atTop: await probe(() => 0, grow),
  }
}

let failed = false
for (const [name, type] of [['chromium', chromium], ['webkit', webkit]]) {
  const browser = await type.launch()
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  const errors = []
  page.on('pageerror', (e) => errors.push(e.message))

  await page.goto(`${BASE}/#/run/${RUN}/kanban`)
  await page.waitForSelector('.kcard', { timeout: 15000 })
  await page.evaluate(() =>
    [...document.querySelectorAll('.kcard')].find((c) => c.textContent.length > 40)?.click())
  await page.waitForSelector('.drawer-body', { timeout: 10000 })
  await page.waitForTimeout(1500)

  const moved = await page.evaluate(measure)
  for (const [when, px] of Object.entries(moved)) {
    const bad = Math.abs(px) > 2
    failed ||= bad
    console.log(`${bad ? 'FAIL' : 'ok  '}  ${name.padEnd(9)} ${when.padEnd(12)} reader moved ${px}px`)
  }
  if (errors.length) { failed = true; console.log(`FAIL  ${name} page errors:`, errors) }
  await browser.close()
}

console.log(failed ? '\nFAILED — the drawer moves under the reader (#20)' : '\nok — drawer holds position in both engines')
process.exit(failed ? 1 : 0)
