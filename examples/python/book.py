"""Steps 1 to 4 of the booking flow: availability, options, hold, then the Stripe link.
Usage: OPTION_CODE=... python3 book.py <activityId> <YYYY-MM-DD>

On the production host this places a real supplier hold and a real Stripe
checkout (the card is authorized only if someone pays it). Point
TOURSCANNER_BASE_URL at a sandbox if you have one.
"""
import json
import os
import sys
import urllib.request

BASE = os.environ.get("TOURSCANNER_BASE_URL", "https://tourscanner.ai")
key = os.environ.get("TOURSCANNER_API_KEY")
if not key:
    sys.exit("Set TOURSCANNER_API_KEY first")
if len(sys.argv) < 3:
    sys.exit("Usage: python3 book.py <activityId> <YYYY-MM-DD>")


def post(path, body):
    req = urllib.request.Request(
        BASE + path,
        data=json.dumps(body).encode(),
        headers={"X-API-Key": key, "Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req) as res:
        return json.load(res)


activity_id, date = int(sys.argv[1]), sys.argv[2]

# 1. Live slots for the date
slots = post("/api/availability/slots", {"activityId": activity_id, "date": date, "adults": 2}).get("slots", [])
slot = next((s for s in slots if s["available"]), None)
if not slot:
    sys.exit(f"Nothing open on {date}")
print("slot:", slot["startTime"], f"{slot['totalPrice'] / 100:.2f} {slot['currency']}")

# 2. Options. The hold needs a productOptionCode from here.
options = post("/api/booking/options", {"activityId": activity_id})
print("options response keys:", ", ".join(options))
option_code = os.environ.get("OPTION_CODE")
if not option_code:
    sys.exit("Pick a productOptionCode from the options above and re-run with OPTION_CODE=...")

# 3. Hold. Reserves the slot and creates the Stripe session.
hold = post("/api/booking/hold", {
    "activityId": activity_id,
    "date": date,
    "startTime": slot["startTime"],
    "productOptionCode": option_code,
    "passengers": {"adults": 2},
    # The traveller's real details. The supplier sends the tickets here.
    "contact": {"firstName": "<first name>", "lastName": "<last name>", "email": "<traveller email>", "phone": "<+ country code and number>"},
    "baseUrl": "http://localhost:3000",
})
print("ref:", hold["internalRef"])
print("price:", f"{hold['retailPrice'] / 100:.2f}", hold["currency"])

# 4. Send the customer here. After they pay, call /api/booking/confirm with the session_id.
print("pay at:", hold["checkoutUrl"])
