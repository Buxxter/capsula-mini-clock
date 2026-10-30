package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const ledBase = "/sys/class/leds"
const illuminancePath = "/sys/bus/iio/devices/iio:device0/in_illuminance_input"

var defaultBrightness uint8 = 255

//	 __ d __
//	|		|
//	c		e
//	|__ g __|
//	|		|
//	b		f
//	|__ a __|
//
// bits order: a b c d e f g
var digitsMap = map[rune]string{
	// : "abcdefg"
	'0': "1111110",
	'1': "0000110",
	'2': "1101101",
	'3': "1001111",
	'4': "0010111",
	'5': "1011011",
	'6': "1111011",
	'7': "0001110",
	'8': "1111111",
	'9': "1011111",
	' ': "0000000",
	'-': "0000001",
}

var segments = []string{"a", "b", "c", "d", "e", "f", "g"}
var digitPositions = []string{"d4", "d3", "d2", "d1"}

// ==
// Illuminance
// ==
func readIlluminanceLux() (float64, error) {
	data, err := os.ReadFile(illuminancePath)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
}

func briForLux(lux float64) uint8 {
	switch {
	case lux < 5:
		return 1
	case lux < 20:
		return 50
	case lux < 100:
		return 120
	default:
		return 255
	}
}

func getBriForLux() (uint8, error) {
	lux, err := readIlluminanceLux()
	if err != nil {
		return 0, err
	}
	return briForLux(lux), nil
}

// ===
// LED
// ===

func writeLed(name string, bri uint8) error {
	path := filepath.Join(ledBase, name, "brightness")
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", bri)), 0644)
}

func setDigit(pos uint8, ch rune, bri uint8) error {
	pattern, ok := digitsMap[ch]
	if !ok {
		pattern = digitsMap[' ']
	}
	ledName := digitPositions[pos]
	for i, seg := range segments {
		var val uint8 = 0
		if pattern[i] == '1' {
			val = bri
		}
		err := writeLed(fmt.Sprintf("clock:%s:%s", ledName, seg), val)
		if err != nil {
			return err
		}
	}
	return nil
}

func setColon(bri uint8) error {
	err := writeLed("clock:colon:top", bri)
	if err != nil {
		return err
	}
	return writeLed("clock:colon:bottom", bri)
}

func setColonWithDefaultBrightness(on bool) error {
	var bri uint8 = 0
	if on {
		bri = defaultBrightness
	}
	return setColon(bri)
}

func renderTime(t time.Time, bri uint8) error {
	s := t.Format("1504") //HHMM
	for i := range uint8(4) {
		val := rune(s[i])
		err := setDigit(i, val, bri)
		if err != nil {
			return err
		}
	}
	return nil
}

func renderTimeWithDefaultBrightness(t time.Time) error {
	return renderTime(t, defaultBrightness)
}

func clearSegment(pos uint8) error {
	return setDigit(pos, ' ', 0)
}

func clear() error {
	for pos := range digitPositions {
		err := clearSegment(uint8(pos))
		if err != nil {
			return err
		}
	}
	return setColon(0)
}

func main() {
	blink := flag.Bool("blink", true, "blink colon every second")
	once := flag.Bool("once", false, "Whrite time once and exit")
	bri := flag.Uint("bri", 0, "Brightness 1-255, 0 - auto (default)")
	flag.Parse()
	autoBri := false

	if *bri == 0 {
		autoBri = true
		fmt.Printf("Autobrightness set to true")
	} else if *bri > 255 {
		defaultBrightness = 255
	} else {
		defaultBrightness = uint8(*bri)
	}

	if *once {
		err := renderTimeWithDefaultBrightness(time.Now())
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	defer clear()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	colonOn := true
	lastMinute := -1
	curBri := defaultBrightness
	lastBri := uint8(0)

	for {
		now := time.Now()
		if autoBri {
			var err error
			curBri, err = getBriForLux()
			if err != nil {
				log.Printf("getBriForLux: %v", err)
			}
		}

		if now.Minute() != lastMinute || curBri != lastBri {

			// err := renderTimeWithDefaultBrightness(now)
			err := renderTime(now, curBri)

			if err != nil {
				log.Printf("renderTime: %v", err)
			} else {
				lastMinute = now.Minute()
			}
		}

		if *blink {
			colonOn = !colonOn
			// err := setColonWithDefaultBrightness(colonOn)
			var err error
			if colonOn {
				err = setColon(curBri)
			} else {
				err = setColon(0)
			}
			if err != nil {
				log.Printf("setColon: %v", err)
			}
		} else if lastBri != curBri {
			err := setColon(curBri)
			if err != nil {
				log.Printf("setColon: %v", err)
			}
		}
		lastBri = curBri
		<-ticker.C
	}
}
