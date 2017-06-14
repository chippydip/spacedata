package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"

	"spacegame/data"
)

func check(e error) {
	if e != nil {
		fmt.Println()
		//panic(e)
		fmt.Println(e)
	}
}

const kmPerAU = 1.496e8
const degToRad = math.Pi / 180

var radii = map[string]float64{ // (assuming albedo of 0.04)
	"Neso":      60, // assuming a mean density of 1.5 g/cm3,[6] its mass is estimated at 2×10^17 kg
	"Halimede":  62,
	"Psamathe":  38,
	"Sao":       44,
	"Laomedeia": 42,
}

func main() {
	//readPdf()

	loadFile("planets.txt", "Sol")
	loadFile("moons.txt", "Sol")
	loadFile("jupiter.txt", "Jupiter")
	loadFile("saturn.txt", "Saturn")
	loadFile("uranus.txt", "Uranus")
	loadFile("neptune.txt", "Neptune")

	//spew.Dump(bodies)

	lut := map[string]data.Orbitable{}
	moons := map[string][]string{}

	for _, b := range bodies {
		if b.radius == 0 {
			b.radius = radii[b.name]
			if b.radius == 0 {
				fmt.Printf("%v missing radius (%v)\n", b.name, b.radius)
				b.radius = 20
			}
		}
		if b.mass == 0 {
			r := b.radius
			rho := 2.5 * 10e-12
			b.mass = math.Pi * r * r * r * rho / 6
		}
		moons[b.primary] = append(moons[b.primary], b.name)
		obt := data.NewOrbit(b.a, b.e, b.pomega(), b.m0(), b.n())
		lut[b.name] = data.NewBody(b.kind, b.name, b.radius/kmPerAU, b.mass, obt)
	}

	lut["Sol"] = data.NewBody(data.Star, "Sol", 695700/kmPerAU, 1.98855e30, data.NewOrbit(0, 0, 0, 0, 0))

	var root data.System
	for len(moons) > 0 {
		fmt.Printf("%v systems remain\n", len(moons))
		cnt := 0
		for p, ms := range moons {
			fmt.Printf("  Checking %v moons of %#v\n", len(ms), p)
			sats := make([]data.Orbitable, 0, len(ms))
			for _, m := range ms {
				if _, has := moons[m]; has {
					fmt.Printf("    %#v's moon %v has moons\n", p, m)
					break
				}
				sats = append(sats, lut[m])
			}
			if len(sats) == len(ms) {
				central := lut[p]
				root = central.(*data.Body).NewSystem(sats)
				central = data.Orbitable(root)
				lut[p] = central

				delete(moons, p)
				cnt++
			} else {
				fmt.Printf("    Failed %v of %v\n", len(sats), len(ms))
			}
		}
		if cnt <= 0 {
			fmt.Println("Failed to generate system")
			spew.Dump(moons)
			break
		}
	}

	fmt.Printf("%#v\n\n", root)

	{
		f, err := os.Create("data/out.json")
		check(err)
		defer f.Close()

		b, err := json.MarshalIndent(root, "", "  ")
		check(err)
		_, err = f.Write(b)
		check(err)
		// out := json.NewEncoder(f)
		// out.Encode(root)
	}

	//spew.Dump(root)

	// fmt.Println(parseTT("2000 Jan 1.5"))
	// fmt.Println(2451545.0)
}

func loadFile(name, primary string) {
	f, err := os.Open("data/" + name)
	check(err)
	defer f.Close()

	fmt.Printf("Processing %v...\n", name)
	parsing := false
	prev := ""
	var tbl [][]string

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if line == "" && prev != "" {
			parsing = !parsing
			if !parsing {
				process(tbl, primary)
				tbl = nil
			}
		}
		if parsing && line != "" {
			cols := strings.Split(line, ", ")
			tbl = append(tbl, cols)
		} else {
			//
		}
		prev = line
	}
	process(tbl, primary)
	check(s.Err())
}

type body struct {
	kind    data.BodyType
	name    string
	primary string
	jd      float64

	// Physical characteristics
	radius    float64
	mass      float64
	density   float64
	rotPeriod float64
	orbPeriod float64
	gravity   float64

	// Orbital parameters
	a  float64
	e  float64
	i  float64
	pw float64
	Ω  float64
	λ  float64

	// Alternate parameters
	w float64
	M float64

	declination float64
	rtAccention float64
	tilt        float64

	// Thermal characteristics
	solarConst  float64
	meanTemp    float64
	albedo      float64
	atmPressure float64
}

func (b *body) pomega() float64 {
	if b.w != 0 || b.M != 0 {
		return b.w * b.M * degToRad
	}
	return b.pw * degToRad
}

func (b *body) m0() float64 {
	return b.λ * degToRad
}

func (b *body) n() float64 {
	if b.orbPeriod != 0 {
		return 360 / (b.orbPeriod * 365.25) * degToRad
	}
	fmt.Printf("Missing P_orb for %v\n", b.name)
	return 0
}

var parseFunc = map[string]func(*body, string){
	"Planet": func(b *body, v string) {
		b.kind = data.Planet
	},
	"Dwarf": func(b *body, v string) {
		b.kind = data.DwarfPlanet
	},
	"JD": func(b *body, v string) {
		b.jd = parse(v)
	},
	"R [km]": func(b *body, v string) {
		b.radius = parse(v)
	},
	"M [10^24 kg]": func(b *body, v string) {
		b.mass = parse(v) * 1e24
	},
	"ρ [kg m^−3]": func(b *body, v string) {
		b.density = parse(v)
	},
	"P_rot [days]": func(b *body, v string) {
		b.rotPeriod = parse(v)
	},
	"P_orb [years]": func(b *body, v string) {
		b.orbPeriod = parse(v)
	},
	"g [m s^−2]": func(b *body, v string) {
		b.gravity = parse(v)
	},

	"a [AU]": func(b *body, v string) {
		b.a = parse(v)
	},
	"e": func(b *body, v string) {
		b.e = parse(v)
	},
	"i [deg]": func(b *body, v string) {
		b.i = parse(v)
	},
	"\x05 [deg]": func(b *body, v string) {
		b.pw = parse(v)
	},
	"Ω [deg]": func(b *body, v string) {
		b.Ω = parse(v)
	},
	"λ [deg]": func(b *body, v string) {
		b.λ = parse(v)
	},

	"S [Wm^−2]": func(b *body, v string) {
		b.solarConst = parse(v)
	},
	"T [K]": func(b *body, v string) {
		b.meanTemp = parse(v)
	},
	"A": func(b *body, v string) {
		b.albedo = parse(v)
	},
	"P [bar]": func(b *body, v string) {
		b.atmPressure = parse(v)
	},

	"Number": nil,
	"M [10^20 kg]": func(b *body, v string) {
		b.mass = parse(v) * 1e20
	},
	"ω [deg]": func(b *body, v string) {
		b.w = parse(v)
	},
	"M [deg]": func(b *body, v string) {
		b.M = parse(v)
	},

	"satellite": func(b *body, v string) {
		b.kind = data.Moon
	},
	"primary": func(b *body, v string) {
		b.primary = v
	},
	"GM [km^3 s^−2]": func(b *body, v string) {
		b.mass = parse(v) * 6.6740831e-2
	},
	"P_orb [days]": func(b *body, v string) {
		b.orbPeriod = parse(v) / 365.25
	},
	"Spin": nil,

	"TT": func(b *body, v string) {
		b.jd = parseTT(v)
	},
	"a [km]": func(b *body, v string) {
		b.a = parse(v) / kmPerAU
	},
	"Dec. [deg]": func(b *body, v string) {
		b.declination = parse(v)
	},
	"R.A. [deg]": func(b *body, v string) {
		b.rtAccention = parse(v)
	},
	"Tilt [deg]": func(b *body, v string) {
		b.tilt = parse(v)
	},

	"JED": func(b *body, v string) {
		b.jd = parse(v)
	},
}

var bodies = map[string]*body{}

func parse(v string) float64 {
	// Convert – to - (these lines are different!)
	v = strings.Replace(v, "−", "-", -1)
	v = strings.Replace(v, "–", "-", -1)

	// Remove ±
	hasPM, err := regexp.MatchString(`\d+(\.\d*)?( ± \d+(\.\d*))?`, v)
	check(err)
	if hasPM {
		v = strings.SplitN(v, " ", 2)[0]
	}

	// Parse
	f, err := strconv.ParseFloat(v, 32)
	check(err)
	return f
}

func parseTT(v string) float64 {
	// 2000 Jan. 1.50
	s := strings.SplitN(v, ".", 2)
	t, err := time.Parse("2006 Jan 2", s[0])
	check(err)

	// Julian date, in seconds, of the "Format" standard time.
	// (See http://www.onlineconversion.com/julian_date.htm)
	const julian = 2453738.4195
	// Easiest way to get the time.Time of the Unix time.
	// (See comments for the UnixDate in package Time.)
	unix := time.Unix(1136239445, 0)
	const oneDay = float64(86400. * time.Second)
	return julian + float64(t.Sub(unix))/oneDay + float64(parse("0."+s[1]))
}

func process(tbl [][]string, primary string) {
	header := tbl[0]
	// for _, c := range header {
	// 	fmt.Printf("%18v", c)
	// }
	// fmt.Println()

	if header[0] == "Rings" {
		return // skip for now
	}

	for _, cols := range tbl[1:] {
		if len(cols) != len(header) {
			fmt.Println(header)
			fmt.Println(cols)
			panic("Not enough columns")
		}
		name := cols[0]
		b, ok := bodies[name]
		if !ok || b == nil {
			b = &body{}
			b.name = name
			b.primary = primary
			b.jd = data.J2000
			bodies[name] = b
		}

		for i, c := range cols {
			f, ok := parseFunc[header[i]]
			if ok {
				if f != nil {
					f(b, c)
				}
			} else {
				check(fmt.Errorf("missing parseFunc: %#v", header[i]))
			}
			//fmt.Printf("%18v", c)
		}
		//fmt.Println()
	}
	//fmt.Println()
}
