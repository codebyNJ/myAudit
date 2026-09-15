import { chromium } from 'playwright'

const RUN = process.env.RUN_ID || '9b0fe6e7-ec6f-4a9f-a32e-b879750511f3'
const BASE = process.env.BASE || 'http://localhost:7788'

const browser = await chromium.launch()
const page = await browser.newPage()
const errors = []
page.on('pageerror', (e) => errors.push(e.message))

await page.goto(`${BASE}/#/run/${RUN}/kanban`, { waitUntil: 'networkidle' })
await page.keyboard.press('Escape') // skip splash if focused

const reviewCard = page.locator('.col-review .kcard[draggable="true"]').first()
const doneCol = page.locator('.col-done .kcards')
await reviewCard.waitFor({ timeout: 15000 })

const title = await reviewCard.locator('.jtitle').innerText()
const cardBox = await reviewCard.boundingBox()
const doneBox = await doneCol.boundingBox()
if (!cardBox || !doneBox) throw new Error('could not measure drag targets')

await page.mouse.move(cardBox.x + 20, cardBox.y + 20)
await page.mouse.down()
await page.mouse.move(doneBox.x + doneBox.width / 2, doneBox.y + 40, { steps: 12 })
await page.mouse.up()
await page.waitForTimeout(1500)

const moved = await page.locator('.col-done .kcard').filter({ hasText: title }).count()
console.log(JSON.stringify({ title, movedToDone: moved > 0, jsErrors: errors }, null, 2))
await browser.close()
process.exit(moved > 0 ? 0 : 1)
