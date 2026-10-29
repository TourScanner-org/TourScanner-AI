# Steps 1 to 4 of the booking flow: availability, options, hold, then the Stripe link.
# Usage: OPTION_CODE=... ruby book.rb <activityId> <YYYY-MM-DD>
#
# On the production host this places a real supplier hold and a real Stripe
# checkout (the card is authorized only if someone pays it). Point
# TOURSCANNER_BASE_URL at a sandbox if you have one.
require 'json'
require 'net/http'
require 'uri'

BASE = ENV.fetch('TOURSCANNER_BASE_URL', 'https://tourscanner.ai')
KEY = ENV['TOURSCANNER_API_KEY'] or abort('Set TOURSCANNER_API_KEY first')
abort('Usage: ruby book.rb <activityId> <YYYY-MM-DD>') if ARGV.length < 2

def post(path, body)
  res = Net::HTTP.post(URI(BASE + path), body.to_json,
                       'X-API-Key' => KEY, 'Content-Type' => 'application/json')
  abort("#{path} -> #{res.code} #{res.body}") unless res.is_a?(Net::HTTPSuccess)
  JSON.parse(res.body)
end

activity_id = ARGV[0].to_i
date = ARGV[1]

# 1. Live slots for the date
slots = post('/api/availability/slots', activityId: activity_id, date: date, adults: 2)['slots'] || []
slot = slots.find { |s| s['available'] } or abort("Nothing open on #{date}")
puts format('slot: %s %.2f %s', slot['startTime'], slot['totalPrice'] / 100.0, slot['currency'])

# 2. Options. The hold needs a productOptionCode from here.
options = post('/api/booking/options', activityId: activity_id)
puts "options response keys: #{options.keys.join(', ')}"
option_code = ENV['OPTION_CODE'] or abort('Pick a productOptionCode from the options above and re-run with OPTION_CODE=...')

# 3. Hold. Reserves the slot and creates the Stripe session.
hold = post('/api/booking/hold',
            activityId: activity_id,
            date: date,
            startTime: slot['startTime'],
            productOptionCode: option_code,
            passengers: { adults: 2 },
            # The traveller's real details. The supplier sends the tickets here.
            contact: { firstName: '<first name>', lastName: '<last name>', email: '<traveller email>', phone: '<+ country code and number>' },
            baseUrl: 'http://localhost:3000')
puts "ref: #{hold['internalRef']}"
puts format('price: %.2f %s', hold['retailPrice'] / 100.0, hold['currency'])

# 4. Send the customer here. After they pay, call /api/booking/confirm with the session_id.
puts "pay at: #{hold['checkoutUrl']}"
