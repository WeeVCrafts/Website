package commerce

import "time"

const ReturnPolicyVersion = "2026-11-23-v2"
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

type PolicySection struct {
	Title   string   `json:"Title"`
	Text    string   `json:"Text,omitempty"`
	Bullets []string `json:"Bullets,omitempty"`
}

func ReturnPolicySections() []PolicySection {
	return []PolicySection{
		{
			Title: "1. Handmade & Handloom Products",
			Bullets: []string{
				"Handmade crafts and handloom products are generally not eligible for change-of-mind returns.",
				"Minor natural variations in colour, texture, weave and finish are part of traditional craftsmanship.",
				"This restriction does not apply to defective, damaged, incorrect or materially misdescribed products.",
			},
		},
		{
			Title: "2. Damaged or Incorrect Products",
			Text:  "If you receive a damaged, defective, incorrect or incomplete product:",
			Bullets: []string{
				"Report the issue to WeevCrafts as soon as possible, preferably within 48 hours of delivery.",
				"Upload clear photographs or an unboxing video showing the parcel, shipping label and product.",
				"Keep the original packaging and product until the complaint is resolved.",
				"We will review your complaint and arrange an eligible replacement or refund.",
			},
		},
		{
			Title: "3. Video Verification – Our Transparency Promise",
			Bullets: []string{
				"Every seller must record a pre-dispatch packing video for each order.",
				"The video will be uploaded to WeevCrafts and shared with the customer through a secure link, including WhatsApp notifications where applicable.",
				"Customers are encouraged to record an unboxing video when opening their parcel.",
				"If an unboxing video is unavailable, other relevant evidence may be considered.",
			},
		},
		{
			Title: "4. Refunds & Replacements",
			Text:  "Approved refunds will be processed to the original payment method wherever possible. Return shipping for verified seller-side or transit-related issues will be arranged without additional cost to the customer, subject to applicable law.",
		},
		{
			Title: "5. Important & Customer Support",
			Text:  "Our video verification process is designed to ensure fairness, transparency and protection for both customers and sellers. Nothing in this policy limits consumer rights available under applicable Indian law.",
			Bullets: []string{
				"For assistance, please contact WeevCrafts Customer Support at support@weevcrafts.com",
			},
		},
	}
}
