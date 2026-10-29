# Booking

The whole flow, with the bodies you actually send. Replace `$KEY` with your API key and `$BASE` with `https://tourscanner.ai`.

## 1. Availability

```sh
curl -s -X POST $BASE/api/availability/slots \
  -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d '{"activityId": 12345, "date": "2026-12-01", "adults": 2}'
```

Returns `slots`, each with a `startTime`, `available`, `pricePerPerson`, `totalPrice` and `currency`, all in minor units. Some slots also carry a `variantId` or `label` when the activity has several options.

Two things to know. On some activities `pricePerPerson` isn't what you pay: when `pricingPackageType` is `UNIT`, one price covers the whole group, so don't multiply it. And if `minTravelers` is set, the real price is for at least that many people.

If the date has no slots, you get an empty list. Ask for the next date rather than retrying.

## 2. Options

```sh
curl -s -X POST $BASE/api/booking/options \
  -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d '{"activityId": 12345}'
```

Gives you the product variants (each with a `productOptionCode`), age bands, booking questions, language guides and the cancellation policy. Show the cancellation policy to your customer before they pay.

## 3. Hold

```sh
curl -s -X POST $BASE/api/booking/hold \
  -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d '{
    "activityId": 12345,
    "date": "2026-12-01",
    "startTime": "09:30",
    "productOptionCode": "FROM_OPTIONS",
    "passengers": { "adults": 2 },
    "contact": {
      "firstName": "<first name>", "lastName": "<last name>",
      "email": "<traveller email>", "phone": "<+ country code and number>"
    },
    "baseUrl": "https://yourapp.com"
  }'
```

Response:

```json
{
  "checkoutUrl": "https://checkout.stripe.com/...",
  "internalRef": "TS-3-abc123-def456",
  "expiresAt": "2026-12-01T15:30:00Z",
  "retailPrice": 5400,
  "currency": "EUR"
}
```

Send the customer to `checkoutUrl`. After they pay, Stripe sends them to `{baseUrl}/booking/success?session_id=...`, so `baseUrl` should be your own origin. Keep `internalRef`, it's how you look the booking up later.

Optional fields: `travelers` (per-person details), `bookingAnswers` for whatever `options` asked, `languageGuide`, and `currency`.

## 4. Confirm

```sh
curl -s -X POST $BASE/api/booking/confirm \
  -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d '{"sessionId": "cs_live_..."}'
```

```json
{
  "status": "confirmed",
  "internalRef": "TS-3-abc123-def456",
  "providerRef": "BR-12345678",
  "voucherUrl": "https://...",
  "ticketUrl": "https://...",
  "activityName": "Colosseum Skip-the-Line Tour",
  "travelDate": "2026-12-01"
}
```

If the supplier rejects the booking at this point, the Stripe authorization is cancelled and the customer isn't charged.

## After booking

`POST /api/booking/status` with `{"internalRef": "..."}` returns the current status and the voucher links. Cancellation and amendment each have a quote endpoint and an action endpoint, see the [docs](https://tourscanner.ai/docs).
