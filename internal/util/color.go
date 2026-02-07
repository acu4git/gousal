package util

import (
	"fmt"
	"math/rand/v2"
)

// HSV色空間を使って鮮やかな色を生成
func RandomBrightColor() string {
	h := rand.Float64() * 360     // 色相: 0-360
	s := 0.7 + rand.Float64()*0.3 // 彩度: 70-100%
	v := 0.8 + rand.Float64()*0.2 // 明度: 80-100%

	return hsvToHex(h, s, v)
}

func hsvToHex(h, s, v float64) string {
	c := v * s
	x := c * (1 - abs(mod(h/60, 2)-1))
	m := v - c

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return fmt.Sprintf("#%02x%02x%02x",
		int((r+m)*255),
		int((g+m)*255),
		int((b+m)*255))
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func mod(x, y float64) float64 {
	return x - y*float64(int(x/y))
}
