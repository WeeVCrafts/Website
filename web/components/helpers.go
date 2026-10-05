package components

import (
	"math"
	"strconv"
)

func wishlistHeaderClass(count int) string {
	if count > 0 {
		return "wishlist-action is-active"
	}
	return "wishlist-action"
}

func wishlistIcon(count int) string {
	if count > 0 {
		return "favorite"
	}
	return "favorite_border"
}

func productHeartClass(saved bool) string {
	if saved {
		return "heart is-active"
	}
	return "heart"
}

func productHeartIcon(saved bool) string {
	if saved {
		return "favorite"
	}
	return "favorite_border"
}

func productHeartLabel(name string, saved bool) string {
	if saved {
		return "Remove " + name + " from wishlist"
	}
	return "Save " + name
}

func productWishlistValues(saved bool) string {
	if saved {
		return `{"source":"wishlist","context":"product-card"}`
	}
	return "{}"
}

func ratingStars(rating string) int {
	value, err := strconv.ParseFloat(rating, 64)
	if err != nil || value <= 0 {
		return 0
	}
	count := int(math.Round(value))
	if count > 5 {
		return 5
	}
	return count
}
