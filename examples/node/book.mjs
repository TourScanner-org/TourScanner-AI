// Steps 1 to 4 of the booking flow: availability, options, hold, then the Stripe link.
// Usage: node book.mjs <activityId> <YYYY-MM-DD>
//
// On the production host this places a real supplier hold and a real Stripe
// checkout (the card is authorized only if someone pays it). Point TOURSCANNER_BASE_URL at a
// sandbox if you have one.
const BASE = process.env.TOURSCANNER_BASE_URL ?? 'https://tourscanner.ai'
const key = process.env.TOURSCANNER_API_KEY
if (!key) throw new Error('Set TOURSCANNER_API_KEY first')

const [activityId, date] = process.argv.slice(2)
if (!activityId || !date) throw new Error('Usage: node book.mjs <activityId> <YYYY-MM-DD>')

async function post(path, body) {
  const res = await fetch(BASE + path, {
    method: 'POST',
    headers: { 'X-API-Key': key, 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`${path} -> ${res.status} ${await res.text()}`)
  return res.json()
}

const id = Number(activityId)

// 1. Live slots for the date
const { slots = [] } = await post('/api/availability/slots', { activityId: id, date, adults: 2 })
const slot = slots.find((s) => s.available)
if (!slot) throw new Error(`Nothing open on ${date}`)
console.log('slot:', slot.startTime, `${(slot.totalPrice / 100).toFixed(2)} ${slot.currency}`)

// 2. Options. The hold needs a productOptionCode from here.
const options = await post('/api/booking/options', { activityId: id })
console.log('options response keys:', Object.keys(options).join(', '))
const optionCode = process.env.OPTION_CODE
if (!optionCode) {
  console.log('Pick a productOptionCode from the options above and re-run with OPTION_CODE=...')
  process.exit(0)
}

// 3. Hold. Reserves the slot and creates the Stripe session.
const hold = await post('/api/booking/hold', {
  activityId: id,
  date,
  startTime: slot.startTime,
  productOptionCode: optionCode,
  passengers: { adults: 2 },
  // The traveller's real details. The supplier sends the tickets here.
  contact: { firstName: '<first name>', lastName: '<last name>', email: '<traveller email>', phone: '<+ country code and number>' },
  baseUrl: 'http://localhost:3000',
})
console.log('ref:', hold.internalRef)
console.log('price:', (hold.retailPrice / 100).toFixed(2), hold.currency)

// 4. Send the customer here. After they pay, call /api/booking/confirm with the session_id.
console.log('pay at:', hold.checkoutUrl)
