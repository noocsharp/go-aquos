package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/noocsharp/go-aquos"
)

func run() (int, error) {
	var port int
	var username string
	var password string
	flag.IntVar(&port, "port", 10002, "TCP port")
	flag.StringVar(&username, "user", "", "Username")
	flag.StringVar(&password, "pass", "", "Password")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage : %s [options] host

options:
`, os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "host is not specified.")
		flag.Usage()

		return 1, nil
	}
	host := args[0]

	client := &aquos.Client{
		Username: username,
		Password: password,
	}

	err := client.Connect(context.Background(), net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return 1, err
	}
	defer client.Close()

	// TV name
	fmt.Printf("TV Name          : %s\n", client.Name())
	// Model name
	fmt.Printf("Model Name       : %s\n", client.ModelName())
	// Software version
	fmt.Printf("Software Version : %s\n", client.SoftwareVersion())
	// Software version
	fmt.Printf("Protocol Version : %s\n", client.IPProtocolVersion())

loop:
	for {
		switch selectCommand() {
		case 0:
			break loop
		case 1:
			err = client.Power(true)
		case 2:
			err = client.Power(false)
		case 3:
			err = client.ToggleInput()
		case 4:
			err = client.ChangeInputTV()
		case 5:
			source := selectInputSource()
			err = client.ChangeInput(source)
		case 6:
			err = client.ChannelUp()
		case 7:
			err = client.ChannelDown()
		case 8:
			volume := selectVolume()
			err = client.SetVolume(volume)
		case 9:
			volume, err := client.Volume()
			if err == nil {
				fmt.Printf("Volume : %d\n", volume)
			}
		default:
			continue
		}
		if err != nil {
			return 1, err
		}
	}

	return 0, nil
}

func selectCommand() (cmd int) {
	for {
		fmt.Print(`
1: Power on
2: Power off
3: Change Input (Toggle)
4: Change Input (TV)
5: Change Input
6: Channel Up
7: Channel Down
8: Change Volume
9: Get Volume
------------------------
0: Exit
> `)
		_, err := fmt.Scan(&cmd)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if cmd < 0 || cmd > 9 {
			fmt.Println("invalid number")
			continue
		}

		break
	}

	return
}

func selectInputSource() (source int) {
	for {
		fmt.Print(`
1: Input 1 (HDMI)
2: Input 2 (HDMI)
3: Input 3 (HDMI)
4: Input 4
> `)
		_, err := fmt.Scan(&source)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if source < 0 {
			fmt.Println("invalid source")
			continue
		}

		break
	}

	return
}

func selectVolume() (volume int) {
	for {
		fmt.Print(`
Volume 0 - 100
> `)
		_, err := fmt.Scan(&volume)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if volume < 0 {
			fmt.Println("invalid volume")
			continue
		}

		break
	}

	return
}

func main() {
	code, err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error : %v\n", err)
	}
	if code != 0 {
		os.Exit(code)
	}
}
