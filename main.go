package main

import (
	"flag"
	"fmt"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/sdl_ttf"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"time"
)

var chip8 *Chip8Emu

var (
	keys = map[sdl.Keycode]int{
		sdl.GetKeyFromName("1"): 0x1,
		sdl.GetKeyFromName("2"): 0x2,
		sdl.GetKeyFromName("3"): 0x3,
		sdl.GetKeyFromName("4"): 0x4,
		sdl.GetKeyFromName("q"): 0x5,
		sdl.GetKeyFromName("w"): 0x6,
		sdl.GetKeyFromName("e"): 0x7,
		sdl.GetKeyFromName("r"): 0x8,
		sdl.GetKeyFromName("a"): 0x9,
		sdl.GetKeyFromName("s"): 0xa,
		sdl.GetKeyFromName("d"): 0xb,
		sdl.GetKeyFromName("f"): 0xc,
		sdl.GetKeyFromName("z"): 0xd,
		sdl.GetKeyFromName("x"): 0xe,
		sdl.GetKeyFromName("c"): 0xf,
		sdl.GetKeyFromName("v"): 0x0,
	}

	chip8_fontset = [80]byte{
		0xF0, 0x90, 0x90, 0x90, 0xF0, // 0
		0x20, 0x60, 0x20, 0x20, 0x70, // 1
		0xF0, 0x10, 0xF0, 0x80, 0xF0, // 2
		0xF0, 0x10, 0xF0, 0x10, 0xF0, // 3
		0x90, 0x90, 0xF0, 0x10, 0x10, // 4
		0xF0, 0x80, 0xF0, 0x10, 0xF0, // 5
		0xF0, 0x80, 0xF0, 0x90, 0xF0, // 6
		0xF0, 0x10, 0x20, 0x40, 0x40, // 7
		0xF0, 0x90, 0xF0, 0x90, 0xF0, // 8
		0xF0, 0x90, 0xF0, 0x10, 0xF0, // 9
		0xF0, 0x90, 0xF0, 0x90, 0x90, // A
		0xE0, 0x90, 0xE0, 0x90, 0xE0, // B
		0xF0, 0x80, 0x80, 0x80, 0xF0, // C
		0xE0, 0x90, 0x90, 0x90, 0xE0, // D
		0xF0, 0x80, 0xF0, 0x80, 0xF0, // E
		0xF0, 0x80, 0xF0, 0x80, 0x80, // F
	}
)

func main() {
	flag.Parse()
	path := flag.Arg(0)
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			fmt.Println("That file does not exists :(")
			return
		}
		fmt.Println("Launching", path)
		run(path)
	} else {
		for {
			fmt.Println("You didnt pass a path to a game to the emulator :(")
			fmt.Print("Please enter a path to a game: ")
			fmt.Scanln(&path)
			if path == "" {
				fmt.Println("You didnt enter a valid path :(")
				continue
			}
			if _, err := os.Stat(path); err != nil {
				fmt.Println("That file does not exists :(")
				continue
			}
			fmt.Println("Launching", path)
			run(path)
			break
		}
	}
}

var paused = false
var running = true
var stepOne = false

func run(gamePath string) {

	window := sdl.CreateWindow("chip 8 emulator", sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		640, 360, sdl.WINDOW_SHOWN)
	renderer := sdl.CreateRenderer(window, -1, sdl.RENDERER_SOFTWARE)
	ttf.Init()
	font, err := ttf.OpenFont("ticketing.ttf", 15)
	if err != nil {
		panic(err)
	}
	sdl.Delay(1000)
	renderer.SetDrawColor(0, 0, 0, 255)
	renderer.Clear()

	chip8 = new(Chip8Emu)

	chip8.init()
	chip8.loadGame(gamePath)

	ticker := time.NewTicker(time.Duration(1000/60) * time.Millisecond)
	opcode := uint16(0)
	pc := uint16(0)

	started := time.Now()
	frames := 0
	fps := 0
	for running {
		<-ticker.C
		if !paused || stepOne {
			opcode, pc = chip8.cycle()

			if chip8.drawFlag {
				chip8.draw(renderer)
			}
			stepOne = false
		} else {
			surface := font.RenderText_Solid("paused press p to unpause\ntest", sdl.Color{200, 50, 50, 255})
			texture := renderer.CreateTextureFromSurface(surface)
			renderer.Copy(texture, &sdl.Rect{0, 0, surface.W, surface.H}, &sdl.Rect{100, 120, 400, 100})
		}

		chip8.handleInput()

		frames++
		if time.Since(started).Seconds() >= 1 {
			fps = frames
			frames = 0
			started = time.Now()
		}

		// Stats
		renderer.SetDrawColor(0, 0, 0, 255)
		renderer.FillRect(&sdl.Rect{0, 330, 640, 30})

		surface := font.RenderText_Solid(fmt.Sprintf("FPS: %2d Last OpCode: 0x%4X Program Counter(pc): %4d", fps, opcode, pc), sdl.Color{200, 50, 50, 255})
		texture := renderer.CreateTextureFromSurface(surface)

		renderer.Copy(texture, &sdl.Rect{0, 0, surface.W, surface.H}, &sdl.Rect{0, 330, 640, 30})
		renderer.Present()
	}
}

type Chip8Emu struct {
	opcode uint16

	memory [4096]byte
	V      [16]byte // General prupose registers, 16th is "carry flag"
	I      uint16   // index register
	pc     uint16   // program counter

	gfx [64 * 32]bool

	delayTimer uint8
	soundTimer uint8

	stack [16]uint16
	sp    uint16 // Stack pointer

	key [16]bool

	drawFlag bool
}

func (c *Chip8Emu) init() {
	// Initialize registers and memory once
	c.pc = 512
	c.I = 0
	c.sp = 0

	// Clear stack
	c.stack = [16]uint16{}
	c.sp = 0

	// Clear registers V0-VF
	c.V = [16]byte{}

	// Clear memory
	c.memory = [4096]byte{}

	// Clear screen
	c.gfx = [64 * 32]bool{}

	// Load fontset
	for i := 0; i < 80; i++ {
		c.memory[i] = chip8_fontset[i]
	}

	// sReset timers
	c.delayTimer = 0
	c.soundTimer = 0
}

func (c *Chip8Emu) loadGame(path string) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	for i := 0; i < len(file); i++ {
		c.memory[512+i] = file[i]
	}
}

func (c *Chip8Emu) cycle() (uint16, uint16) {
	// Fetch Opcode
	opcode := uint16(c.memory[c.pc])<<8 | uint16(c.memory[c.pc+1])
	//log.Printf("0x%X", opcode)
	// X is 4 bits after the opcode
	// Y is 8 bits after the opcode
	x := opcode & 0x0f00 >> 8
	y := opcode & 0x00f0 >> 4
	vx := c.V[x]
	vy := c.V[y]

	tempPc := c.pc
	c.pc += 2
	switch opcode & 0xf000 {
	case 0x0000:
		switch opcode {
		case 0x00e0: // Clear the screen
			c.gfx = [64 * 32]bool{}
			c.drawFlag = true
		case 0x00ee: // Return from subroutine
			c.sp -= 1
			c.pc = c.stack[c.sp] + 2
			c.stack[c.sp] = 0
		}
	case 0x1000: // Jumps to address NNN
		c.pc = uint16(opcode & 0x0fff)
	case 0x2000: // Call subtroutine
		c.stack[c.sp] = tempPc
		c.sp++
		c.pc = uint16(opcode & 0x0fff)
	case 0x3000, 0x4000:
		nn := byte(opcode & 0x00ff)
		if opcode&0xf000 == 0x3000 { // Skips the next instruction if VX equals NN.
			if vx == nn {
				c.pc += 2
			}
		} else { // Skips the next instruction if VX doesn't equal NN.
			if vx != nn {
				c.pc += 2
			}
		}
	case 0x5000: // Skips the next instruction if VX equals VY.
		if vx == vy {
			c.pc += 2
		}
	case 0x6000: // Sets vx to nn
		c.V[x] = byte(opcode & 0x00ff)
	case 0x7000: // Adds NN to VX.
		c.V[x] += byte(opcode & 0x00ff)
	case 0x8000:
		switch opcode & 0x000f {
		case 0x0: // vx = vy
			c.V[x] = vy
		case 0x1: // vx = or
			c.V[x] = vx | vy
		case 0x2: // vx = and
			c.V[x] = vx & vy
		case 0x3: // vx = xor
			c.V[x] = vx ^ vy
		case 0x4: // Adds VY to VX. VF is set to 1 when there's a carry, and to 0 when there isn't.
			if int(vx)+int(vy) > 255 {
				c.V[0xf] = 1
			} else {
				c.V[0xf] = 0
			}
			c.V[x] += vy
		case 0x5: // VY is subtracted from VX. VF is set to 0 when there's a borrow, and 1 when there isn't.
			if vy > vx {
				c.V[0xf] = 0
			} else {
				c.V[0xf] = 1
			}
			c.V[x] -= vy
		case 0x6: // Shifts VX right by one. VF is set to the value of the least significant bit of VX before the shift
			c.V[0xf] = vx & 0x1
			c.V[x] = vx >> 1
		case 0x7: // Sets VX to VY minus VX. VF is set to 0 when there's a borrow, and 1 when there isn't.
			if vx > vy {
				c.V[0xf] = 0
			} else {
				c.V[0xf] = 1
			}
			c.V[x] = vy - vx
		case 0xe: // Shifts VX left by one. VF is set to the value of the most significant bit of VX before the shift.[2]
			c.V[0xf] = vx >> 7
			c.V[x] = vx << 1
		}

	case 0x9000: // Skips the next instruction if VX doesn't equal VY
		if vx != vy {
			c.pc += 2
		}
	case 0xa000: // ANNN: Sets I to the address NNN
		c.I = opcode & 0x0fff
	case 0xb000: // Jumps to the address NNN plus V0
		nnn := opcode & 0x0fff
		v0 := c.V[0]
		c.pc = nnn + uint16(v0)
	case 0xc000: // Sets VX to a random number AND NN.
		rnum := uint8(rand.Intn(255))
		nn := opcode & 0x00ff
		c.V[x] = rnum & uint8(nn)
	case 0xd000: // DXYN Draw sprite
		xpos := int(vx)
		ypos := int(vy)
		height := int(opcode & 0x000f)

		c.V[0xf] = 0
		for yline := 0; yline < height; yline++ {
			spriteLine := c.memory[int(c.I)+yline]
			for xline := 0; xline < 8; xline++ {
				masked := spriteLine & (0x80 >> uint8(xline))
				if masked != 0 {
					index := xpos + xline + ((ypos + yline) * 64)
					if index >= len(c.gfx) {
						continue
					}
					if c.gfx[index] {
						c.V[0xf] = 1
						c.gfx[index] = false
					} else {
						c.gfx[index] = true
					}
				}
			}
		}
		c.drawFlag = true

	case 0xe000:
		pressed := c.key[vx]
		switch opcode & 0x00ff {
		case 0x009e: // Skips the next instruction if the key stored in VX is pressed
			if pressed {
				c.pc += 2
			}
		case 0x00a1: // Skips the next instruction if the key stored in VX isn't pressed.
			if !pressed {
				c.pc += 2
			}
		}
	case 0xf000:
		switch opcode & 0x00ff {
		case 0x07: // Sets VX to the value of the delay timer.
			c.V[x] = c.delayTimer
		case 0x0a: // A key press is awaited, and then stored in VX.
			ticker := time.NewTicker(time.Duration(1) * time.Millisecond)
			log.Println("Awaiting input before continuing")
		KEYWAITLOOP:
			for {
				for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
					switch t := event.(type) {
					case *sdl.KeyUpEvent:
						kc := t.Keysym.Sym
						if rc, ok := keys[kc]; ok {
							c.key[rc] = false
						}
					case *sdl.KeyDownEvent:
						kc := t.Keysym.Sym
						if rc, ok := keys[kc]; ok {
							c.key[rc] = true
							c.V[x] = byte(rc)
							break KEYWAITLOOP
						}
					}
				}
				<-ticker.C
			}
			ticker.Stop()
		case 0x15: // Sets the delay timer to VX.
			c.delayTimer = vx
		case 0x18: // Sets the sound timer to VX.
			c.soundTimer = vx
		case 0x1e: // Adds VX to I
			c.I += uint16(vx)
		case 0x29: // Sets I to the location of the sprite for the character in VX. Characters 0-F (in hexadecimal) are represented by a 4x5 font.
			c.I = uint16(c.V[x]) * 5
		case 0x33: // Stores the Binary-coded decimal representation of VX, with the most significant of three digits at the address in I, the middle digit at I plus 1, and the least significant digit at I plus 2. (In other words, take the decimal representation of VX, place the hundreds digit in memory at location in I, the tens digit at location I+1, and the ones digit at location I+2.)   }
			c.memory[c.I] = byte(vx / 100 % 10)
			c.memory[c.I+1] = byte(vx / 10 % 10)
			c.memory[c.I+2] = byte(vx % 10)

		case 0x55: // Stores V0 to VX in memory starting at address I.
			for i := 0; i < int(x); i++ {
				c.memory[int(c.I)+i] = c.V[i]
			}

		case 0x65: // Fills V0 to VX with values from memory starting at address I.
			for i := 0; i < int(x); i++ {
				c.V[i] = c.memory[int(c.I)+i]
			}

		}
	default:
		log.Println("Uknown opcode", opcode)
	}

	if c.delayTimer > 0 {
		c.delayTimer -= 1
	}

	if c.soundTimer > 0 {
		if c.soundTimer == 1 {
			log.Println("Beep... make your computer actually beep later i guess...")
			c.soundTimer -= 1
		}
	}

	return opcode, tempPc
}

func (c *Chip8Emu) draw(renderer *sdl.Renderer) {
	renderer.SetDrawColor(0, 0, 0, 255)
	renderer.Clear()
	renderer.SetDrawColor(0, 255, 155, 255)
	for x := 0; x < 64; x++ {
		for y := 0; y < 32; y++ {
			index := x + (y * 64)
			if !c.gfx[index] {
				continue
			}
			rx := int32(x * 10)
			ry := int32(y * 10)
			rect := &sdl.Rect{rx, ry, 10, 10}
			renderer.FillRect(rect)
		}
	}
	c.drawFlag = false
}

func (c *Chip8Emu) handleInput() {
	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		switch t := event.(type) {
		case *sdl.KeyUpEvent:
			kc := t.Keysym.Sym
			if rc, ok := keys[kc]; ok {
				c.key[rc] = false
				log.Println("Released key", rc)
			}
		case *sdl.KeyDownEvent:
			kc := t.Keysym.Sym
			if rc, ok := keys[kc]; ok {
				if !c.key[rc] {
					c.key[rc] = true
					log.Println("Pressed key", rc)
				}

			} else {
				switch kn := sdl.GetKeyName(kc); kn {
				case "P":
					paused = !paused
				case "Escape":
					running = false
				default:
					if paused {
						stepOne = true
					}
				}
			}
		case *sdl.QuitEvent:
			running = false
		}
	}
}
