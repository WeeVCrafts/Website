package commerce

import "time"

const ReturnPolicyVersion = "2026-11-16-v1"
const ReversePickupFeeCents int64 = 20000

func ClaimWithinWindow(kind string, delivered, submitted time.Time) bool {
	if delivered.IsZero() || submitted.Before(delivered) {
		return false
	}
	window := 48 * time.Hour
	if kind == "change_of_mind" {
		window = 72 * time.Hour
	}
	return !submitted.After(delivered.Add(window))
}

// Calendar expiry clamps month-end instead of Go's date normalization into
// the following month. A credit issued on August 31 expires on February's last day.
func CreditExpiry(issued time.Time) time.Time {
	target := issued.AddDate(0, 6, -issued.Day()+1)
	last := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, target.Location()).Day()
	day := min(issued.Day(), last)
	return time.Date(target.Year(), target.Month(), day, issued.Hour(), issued.Minute(), issued.Second(), issued.Nanosecond(), issued.Location())
}

func RestoredCreditExpiry(original, restored time.Time) time.Time {
	return maxTime(original, restored.Add(30*24*time.Hour))
}
func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// Operations follow the published customer-care schedule: Monday–Saturday,
// 09:00–19:00 IST. Courier dates remain the provider's actual dates.
func BusinessDeadline(start time.Time, hours int) time.Time {
	if hours <= 0 {
		return start
	}
	current := start.In(time.FixedZone("IST", 19800))
	left := time.Duration(hours) * time.Hour
	for left > 0 {
		opening := time.Date(current.Year(), current.Month(), current.Day(), 9, 0, 0, 0, current.Location())
		closing := opening.Add(10 * time.Hour)
		if current.Weekday() == time.Sunday || !current.Before(closing) {
			current = opening.AddDate(0, 0, 1)
			continue
		}
		if current.Before(opening) {
			current = opening
		}
		step := min(left, closing.Sub(current))
		current = current.Add(step)
		left -= step
	}
	return current
}

type PolicySection struct{ Title, Text string }

func ReturnPolicySections() []PolicySection {
	return []PolicySection{
		{"Our commitment to authentic craftsmanship", "WeevCrafts brings together handmade items, fine arts, traditional crafts and handlooms from independent Indian artisans. Subtle variations in weave, yarn tension, texture, brush strokes, motif placement and natural dye shades are inherent handmade characteristics, rather than defects."},
		{"Damage, defects and wrong items", "Report transit damage, a verified manufacturing or weaving defect, or an incorrect item within 48 hours of delivery. Approved claims receive a free replacement. If a suitable replacement is unavailable, we refund the eligible amount to the original payment method. This refund fallback also applies to fragile items. Partial claims cover affected items; original delivery charges are refunded once all items in the order qualify."},
		{"Unboxing evidence", "Please provide a continuous, unedited 360-degree unboxing video showing the sealed outer package, shipping label and opening of the parcel. Submit your claim within 48 hours even if evidence needs further review. Company staff can accept alternative evidence in exceptional cases and will record the reason for that decision."},
		{"Change of mind and preference", "For eligible textiles and crafts, request a return within three days of delivery. Items must be unused, unwashed, unworn and unaltered, with original tags, artisan provenance cards and packaging intact. Approved returns receive account-bound store credit for the discounted item amount, less ₹200 once per approved return pickup. Original delivery charges are excluded. We show the expected credit before you proceed; no negative credit or additional return charge is imposed."},
		{"Using store credit", "Store credit is valid for six calendar months from issuance and can be used partly or fully across WeevCrafts sellers. It is nontransferable and cannot be withdrawn as cash. Pay any remaining checkout amount through Razorpay. If a qualifying cancellation or refund restores redeemed credit, its validity is the original expiry or 30 days after restoration, whichever is later. Only the portion paid through Razorpay is refunded through Razorpay."},
		{"Fragile fine art, paintings and sculptures", "Fragile fine art, paintings and sculptures are excluded from change-of-mind store-credit returns because of transit breakage risk. Eligible damage, defect and wrong-item claims remain available, with free replacement or a refund if a suitable replacement is unavailable."},
		{"When returns are unavailable", "Slight colour differences caused by lighting or screen calibration, natural yarn slubs, minor handloom irregularities and organic imperfections in craft materials are not defects. Claims for used, stained, washed, altered or tagless items, perfume or detergent odour, damage after delivery, or failure to follow care instructions are not eligible under this policy."},
		{"Order cancellations", "You may cancel for a full original-payment refund within 12 hours of placing the order or before dispatch, whichever is earlier. Dispatch means actual courier handover, not merely generating an AWB or label. Pre-dispatch quality-check evidence is retained with the order. After dispatch, contact support about a delivery or product issue."},
		{"Review, collection and inspection", "Our quality team reviews claims within 24–48 business hours. Approved reverse pickups are arranged within 2–4 business days, subject to courier serviceability. Once returned items pass inspection, eligible store credit or original-payment refunds are processed within 5–7 business days. Your request shows actual progress; submitting a claim does not itself approve a return, replacement or refund."},
	}
}
