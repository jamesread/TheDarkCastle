package main

import (
	"bufio"
	"fmt"
	"github.com/gookit/color"
	gettext "github.com/gosexy/gettext"
	"github.com/zyedidia/generic/mapset"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"
	"regexp"
)

var (
	stdinreader      *bufio.Reader
	colorCell        color.Style
	colorAction      color.Style
	colorActionShort color.Style
	colorDenied      color.Style
	colorItem        color.Style
	colorSubtle      color.Style
	colorPlayer      color.Style
	roomDescriptions []string
	regexpStringFunctions *regexp.Regexp
)

func init() {
	roomDescriptions = []string{
		gettext.Gettext("ROOM_COBBLESTONE"),
		gettext.Gettext("ROOM_COURTYARD"),
		gettext.Gettext("ROOM_GARDEN"),
		gettext.Gettext("ROOM_WORKSHOP"),
		gettext.Gettext("ROOM_KITCHEN"),
		gettext.Gettext("ROOM_BANQUET"),
	}

	regexpStringFunctions = regexp.MustCompile(`([a-zA-Z_]*){([a-z A-Z0-9_,:]+)}`)
}

const (
	NORTH int = 0
	EAST      = 1
	SOUTH     = 2
	WEST      = 3

	//	PLAYER_ICON = "\xf0\x9f\x91\xa8"
	PLAYER_ICON = "+"
	ICON_BLOCK = "\u2588"
)

type Grid struct {
	roomMap map[int]map[int]*Cell
	roomDir map[string]*Cell
	rows    int
	cols    int

	startCell *Cell
	exitCell  *Cell
}



func formatString(msg string, a ...any ) string {
	ret := fmt.Sprintf(msg, a...)

	matches := regexpStringFunctions.FindAllStringSubmatch(ret, -1)

	for _, match := range matches {
		function := match[1]
		operand := match[2]

		val := "blat"

		switch(function) {
		case "GT":
			val = gettext.Gettext(operand)
			break
		case "ITEM":
			val = colorItem.Sprintf(operand)
			break
		case "ROOM":
			val = colorCell.Sprintf(gettext.Gettext(operand))
			break
		case "ACTION":
			val = colorActionShort.Sprintf(operand[0:1]) + colorAction.Sprintf(operand[1:])
			break
		default:
			ret = fmt.Sprintf("ERROR, function not found: %v -> %v", function, operand)
		}

		ret = strings.Replace(ret, match[0], val, -1)
	}

	return ret
}

func printString(msg string, a ...any) {
	fmt.Print(formatString(msg, a...))
}

func (g *Grid) getCellRelative(r *Cell, dir int) *Cell {
	rowRelative, colRelative := dir2rel(dir)

	return g.getCell(r.row+rowRelative, r.col+colRelative)
}

func (g *Grid) getCell(row int, col int) *Cell {
	if row < 0 || row > g.rows-1 || col < 0 || col > g.cols-1 {
		return nil
		//log.Fatalf("Could not get: row:%v, col:%v", row, col)
	}

	r, found := g.roomMap[row][col]

	if !found {
		panic("getCell is nil")
	}

	return r
}

func generateCellDescription() string {
	i := rand.Intn(len(roomDescriptions))

	return roomDescriptions[i]
}

func (g *Grid) build(rows int, cols int) {
	g.rows = rows
	g.cols = cols

	g.roomMap = make(map[int]map[int]*Cell, rows)
	g.roomDir = make(map[string]*Cell)

	for currentRow := 0; currentRow < rows; currentRow++ {
		g.roomMap[currentRow] = make(map[int]*Cell)

		for currentCol := 0; currentCol < cols; currentCol++ {
			roomName := fmt.Sprintf("%v:%v", currentRow, currentCol)

			r := &Cell{
				name:          roomName,
				description:   generateCellDescription(),
				visited:       false,
				requiredItems: mapset.New[*Item](),
				itemsOnFloor:  mapset.New[*Item](),
				row:           currentRow,
				col:           currentCol,
			}

			g.roomMap[currentRow][currentCol] = r
			g.roomDir[roomName] = r
		}
	}
}

func smallerInt(a int, b int) int {
	if a < b {
		return a
	}

	return b
}

func dir2rel(dir int) (int, int) {
	rowRelative := 0
	colRelative := 0

	switch dir {
	case NORTH:
		rowRelative = -1
		colRelative = 0
		break
	case EAST:
		rowRelative = 0
		colRelative = 1
		break
	case SOUTH:
		rowRelative = 1
		colRelative = 0
		break
	case WEST:
		rowRelative = 0
		colRelative = -1
		break
	default:
		panic("Direction unknown")
	}

	return rowRelative, colRelative
}

func randomDir() int {
	return rand.Intn(4)
}

func buildLineOfRooms(g *Grid, row int, col int, dir int, branchProbability float32) (int, int) {
	if dir == -1 {
		dir = randomDir()
	}
	
	rowRelative, colRelative := dir2rel(dir)

	distance := 2 + rand.Intn(3)

	for segment := 0; segment < distance; segment++ {
		current := g.getCell(row, col)
		current.room = true

		// If the next cell would be null, we are at the edge of the grid.
		if g.getCell(row + rowRelative, col + colRelative) == nil {
			return row, col
		}

		if rand.Float32() < branchProbability {
			buildLineOfRooms(g, row, col, -1, branchProbability-.1)
		}

		row += rowRelative
		col += colRelative
	}

	return row, col
}

func addCandidateIfNotVisited(visited *mapset.Set[*Cell], candidates *mapset.Set[*Cell], avoid *mapset.Set[*Cell], candidate *Cell) {
	if candidate == nil {
		return
	}

	if visited.Has(candidate) {
		return
	}

	if avoid.Has(candidate) {
		return
	}

	candidates.Put(candidate)
}

func dfsWalkToRandom(from *Cell, visited *mapset.Set[*Cell], avoid *mapset.Set[*Cell]) *Cell {
	visited.Put(from)

	candidates := mapset.New[*Cell]()

	addCandidateIfNotVisited(visited, &candidates, avoid, from.north)
	addCandidateIfNotVisited(visited, &candidates, avoid, from.east)
	addCandidateIfNotVisited(visited, &candidates, avoid, from.south)
	addCandidateIfNotVisited(visited, &candidates, avoid, from.west)

	var ret *Cell

	candidates.Each(func(from *Cell) {
		if from == nil {
			panic("nil candidate")
		}

		if rand.Float32() < .1 {
			ret = from
		} else {
			r := dfsWalkToRandom(from, visited, avoid)

			if r != nil {
				ret = r
			}
		}
	})

	fmt.Printf("Walk: %v %v %v\n", from.name, candidates.Size(), ret)

	return ret
}

func (g *Game) dfsPlace(start *Cell, item *Item, avoid1 *Cell) *Cell {
	visited := mapset.New[*Cell]()
	avoid := mapset.New[*Cell]()
	avoid.Put(avoid1)

	room := dfsWalkToRandom(start, &visited, &avoid)

	if room == nil {
		panic("Walk to random = nil")
	}

	room.itemsOnFloor.Put(item)

	g.hints = append(g.hints, "The "+colorDenied.Sprintf(item.name)+" is in "+colorCell.Sprintf(room.name))

	return room
}

func generateGrid() *Grid {
	g := &Grid{}
	g.build(10, 20)

	row := 5
	col := 10

	g.startCell = g.roomMap[row][col]

	buildLineOfRooms(g, row, col, NORTH, .5)
	buildLineOfRooms(g, row, col, EAST, .5)
	buildLineOfRooms(g, row, col, SOUTH, .5)
	exitCellRow, exitCellCol := buildLineOfRooms(g, row, col, WEST, .5)

	g.exitCell = g.roomMap[exitCellRow][exitCellCol]
	g.exitCell.exitCell = true

	g.buildAllCellConnections()

	return g
}

func (g *Grid) buildAllCellConnections() {
	for row := 0; row < g.rows; row++ {
		for col := 0; col < g.cols; col++ {
			buildCellConnections(g, g.roomMap[row][col])
		}
	}
}

func buildCellConnections(g *Grid, current *Cell) {
	for dir := 0; dir < 4; dir++ {
		adj := g.getCellRelative(current, dir)

		if adj == nil {
			continue
		}

		switch dir {
		case NORTH:
			current.north = adj
			adj.south = current
			break
		case EAST:
			current.east = adj
			adj.west = current
			break
		case SOUTH:
			current.south = adj
			adj.north = current
			break
		case WEST:
			current.west = adj
			adj.east = current
			break
		default:
			panic("Direction unknown")
		}
	}
}

type ItemSet = mapset.Set[*Item]

type Game struct {
	currentCell *Cell

	hints []string

	grid *Grid

	hasMap bool

	ownedItems ItemSet
}

type Item struct {
	name string
}

type Cell struct {
	name        string
	description string

	row int
	col int

	itemsOnFloor  ItemSet
	requiredItems ItemSet

	north *Cell
	east  *Cell
	south *Cell
	west  *Cell

	visited bool
	discovered bool
	room bool

	exitCell bool
}

func (g *Game) getRelative(r *Cell, dir int) *Cell {
	rowRel, colRel := dir2rel(dir)

	return g.grid.getCell(r.row+rowRel, r.col+colRel)
}

func buildGame() *Game {
	game := &Game{
		grid:       generateGrid(),
		ownedItems: mapset.New[*Item](),
		hasMap:     false,
	}

	exitKey := &Item{
		name: "Red Key",
	}

	game.grid.exitCell.requiredItems.Put(exitKey)

	game.dfsPlace(game.grid.startCell, exitKey, game.grid.exitCell)
	game.dfsPlace(game.grid.startCell, &Item{name: "Map"}, game.grid.exitCell)

	game.currentCell = game.grid.roomMap[game.grid.rows/2][game.grid.cols/2]

	game.moveCell(game.grid.startCell)

	return game
}

func (g *Game) findCell(name string) *Cell {
	r, found := g.grid.roomDir[name]

	if !found {
		log.Printf("Cannot find room: %v", name)
	}

	return r
}

func (g *Game) canEnter(r *Cell, printReason bool) (bool, *ItemSet) {
	missingItems := mapset.New[*Item]()

	if !r.room {
		if printReason {
			fmt.Println("There is nothing in that direction.")
		}

		return false, &missingItems
	}

	r.requiredItems.Each(func(reqItem *Item) {
		if g.ownedItems.Has(reqItem) {
		} else {
			missingItems.Put(reqItem)
		}
	})

	if missingItems.Size() > 0 && printReason {
		missingItems.Each(func(i *Item) {
			fmt.Printf("To enter, you need: %v \n", colorDenied.Sprintf(i.name))
		})
	}

	return missingItems.Size() == 0, &missingItems
}

func (g *Game) moveCell(requestedCell *Cell) {
	if res, _ := g.canEnter(requestedCell, true); res {
		fmt.Printf(gettext.Gettext("OPEN_DOOR")+"%v\n\n", colorCell.Sprintf(gettext.Gettext(requestedCell.description)))

		requestedCell.visited = true

		for dir := 0; dir < 4; dir++ {
			adj := g.getRelative(requestedCell, dir)
			adj.discovered = true
		}

		g.currentCell = requestedCell
	}
}

func (r *Cell) hasConnections() bool {
	return r.north == nil || r.east == nil || r.south == nil || r.west == nil
}

func (g *Game) printMap() {
	indent := "\t\t\t"

	printString(indent)
	printStringCenter(g.getDirectionActionText(g.currentCell.north, "North"))
	fmt.Println("")
	fmt.Println("")

	for row := 0; row < g.grid.rows; row++ {
		if row == g.currentCell.row {
			txt := formatString(g.getDirectionActionText(g.currentCell.west, "West"))
			l := 24 - len(color.ClearCode(txt))

			if (l > 0) {
				printString(strings.Repeat(" ", l))
			}
			printString(txt)
		} else {
			fmt.Printf(indent)
		}

		for col := 0; col < g.grid.cols; col++ {
			r := g.grid.roomMap[row][col]

			if g.currentCell == r {
				fmt.Printf(colorPlayer.Sprint(PLAYER_ICON))
			} else {
				if r.visited {
					fmt.Printf(colorCell.Sprintf("v"))
				} else if r.discovered {
					if r.room {
						fmt.Printf(colorSubtle.Sprintf("o"))
					} else {
						fmt.Printf(colorSubtle.Sprintf(ICON_BLOCK))
					}
				} else {
					if g.hasMap {
						if !r.hasConnections() {
							fmt.Printf(colorSubtle.Sprintf("."))
						} else {
							if r.exitCell {
								fmt.Printf(colorDenied.Sprintf("£"))
							} else {
								fmt.Printf(colorSubtle.Sprintf("."))
							}
						}
					} else {
						fmt.Printf(colorSubtle.Sprintf("."))
					}
				}
			}
		}

		if row == g.currentCell.row {
			printString(" %s", g.getDirectionActionText(g.currentCell.east, "East"))
		}

		fmt.Print("\n")
	}

	fmt.Println("")

	printString(indent)
	printStringCenter(g.getDirectionActionText(g.currentCell.south, "South"))

	fmt.Println("")
	fmt.Println("")
}

func printStringCenter(s string) {
	w := 28 + (len(s) - len(color.ClearCode(s)))
	printString("%[1]*s", -w, fmt.Sprintf("%[1]*s", (w + len(s))/2, s))
}

func (g *Game) processInput(in string) {
	fmt.Printf("\n")

	if in == "h" || in == "hint" {
		idx := rand.Intn(len(g.hints))
		fmt.Printf(g.hints[idx] + "\n")
		return
	}

	if in == "i" || in == "items" || in == "inv" || in == "inventory" {
		itemCount := g.ownedItems.Size()
		fmt.Printf(gettext.NGettext("ITEM_INVENTORY", "%d ITEM_INVENTORY_PL", uint64(itemCount))+"\n\n", itemCount)

		g.ownedItems.Each(func(item *Item) {
			fmt.Printf("Item: %v\n", item.name)
		})

		return
	}

	if in == "quit" || in == "q" {
		fmt.Println(gettext.Gettext("GOODBYE"))
		os.Exit(0)
	}

	if in == "east" || in == "e" {
		g.moveCell(g.currentCell.east)
		return
	}

	if in == "west" || in == "w" {
		g.moveCell(g.currentCell.west)
		return
	}

	if in == "north" || in == "n" {
		g.moveCell(g.currentCell.north)
		return
	}

	if in == "south" || in == "s" {
		g.moveCell(g.currentCell.south)
		return
	}

	fmt.Printf(gettext.Gettext("UNKNOWN_COMMAND") + "\n\n")
}

func clr() {
	c := exec.Command("clear")
	c.Stdout = os.Stdout
	c.Run()
}

func setJoin(set *ItemSet) string {
	ret := ""

	set.Each(func(i *Item) {
		ret += i.name + ","
	})


	// Remove trailing comma
	if len(ret) > 0 {
		ret = ret[:len(ret)-1]
	}

	return ret
}

func (game *Game) getDirectionActionText(c *Cell, direction string) string {
	if !c.room {
		return colorSubtle.Sprintf("# Wall #")
	}

	lockedText := ""

	if canEnter, missingItems := game.canEnter(c, false); !canEnter {
		lockedText = colorDenied.Sprintf(" (%v)", setJoin(missingItems))
	}

	//roomDescription := colorSubtle.Sprintf("unvisited")

	if c.visited {
		//	roomDescription = colorCell.Sprintf(gettext.Gettext(room.description))
	}

	return fmt.Sprintf("ACTION{%v}: (%v) %v", direction, colorCell.Sprintf(c.name), lockedText)
}

func (game *Game) printPossibleActions() {
	printBullet("ACTION{Inventory}: \tShow inventory")
	printBullet("ACTION{Hint}: \tShow hint")
}

func printBullet(txt string) {
	fmt.Printf("- " + formatString(txt) + "\n")
}

func initGettext() {
	gettext.BindTextdomain("default", "mo/")
	gettext.Textdomain("default")

	gettext.SetLocale(gettext.LcAll, "en_GB.utf8")
}

func initColors() {
	colorCell = color.Style{color.FgBlue, color.OpBold}
	colorAction = color.Style{color.FgMagenta}
	colorActionShort = color.Style{color.FgMagenta, color.OpBold}
	colorDenied = color.Style{color.FgRed, color.OpBold}
	colorItem = color.Style{color.FgGreen, color.OpBold}
	colorSubtle = color.Style{color.FgGray, color.OpBold}
	colorPlayer = color.Style{color.FgGreen, color.BgBlack, color.OpBold}
}

func main() {
	initGettext()
	initColors()

	rand.Seed(time.Now().UnixNano())

	game := buildGame()

	clr()

	for {
		mainLoop(game)
	}
}

func mainLoop(game *Game) {
	if game.currentCell.exitCell {
		fmt.Printf(gettext.Gettext("EXIT") + "\n\n")
		return
	}

	printString("GT{IN_ROOM} ROOM{%v} (ROOM{%v})\n\n", game.currentCell.description, game.currentCell.name)

	//	color.Red.Printf(PLAYER_ICON)

	game.currentCell.itemsOnFloor.Each(func(item *Item) {
		game.ownedItems.Put(item)
		game.currentCell.itemsOnFloor.Remove(item)

		if item.name == "Map" {
			game.hasMap = true
		}

		colorItem.Sprintf("Picked up item %v \n\n", item.name)
	})

	game.printMap()

	game.printPossibleActions()

	fmt.Printf("\n> ")

	game.processInput(getInput())

	fmt.Printf(gettext.Gettext("ENTER_CONTINUE"))

	getInput()

	clr()
}

func getInput() string {
	if stdinreader == nil {
		stdinreader = bufio.NewReader(os.Stdin)
	}

	chr, err := stdinreader.ReadString('\n')

	if err != nil {
		log.Fatalf("Cannot read stdin: %v", err)
		return ""
	}

	return strings.Trim(chr, "\n")
}
