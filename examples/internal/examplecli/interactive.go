package examplecli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Command string

const (
	CommandResolvePools   Command = "resolve-pools"
	CommandGetPool        Command = "get-pool"
	CommandGetTokenMarket Command = "get-token-market"
	CommandAllMethods     Command = "all-methods"
	CommandBatchAll       Command = "batch-all"
	CommandInteractive    Command = "interactive"
)

func (c Command) String() string {
	return string(c)
}

func RunCommand(ctx context.Context, runner *Runner, command Command, mint string, pool string, protocol string) error {
	switch command {
	case CommandResolvePools:
		return runner.ResolvePools(ctx, mint)
	case CommandGetPool:
		return runner.GetPool(ctx, pool)
	case CommandGetTokenMarket:
		return runner.GetTokenMarket(ctx, mint)
	case CommandAllMethods:
		return runner.RunAllPublicMethods(ctx, mint, protocol)
	case CommandBatchAll:
		for _, sampleMint := range SampleMints {
			if err := runner.RunAllPublicMethods(ctx, sampleMint, protocol); err != nil {
				fmt.Printf("batch-all mint=%s error=%v\n", sampleMint, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func RunInteractive(ctx context.Context, runner *Runner) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("solana-token-market examples interactive CLI")
	fmt.Println("Available commands: resolve-pools, get-pool, get-token-market, all-methods, batch-all")
	for i, mint := range SampleMints {
		fmt.Printf("[%d] %s\n", i+1, mint)
	}

	commandInput, err := prompt(reader, "Command", string(CommandAllMethods))
	if err != nil {
		return err
	}
	command := Command(strings.TrimSpace(commandInput))

	mint := ""
	pool := ""
	protocol := ""

	switch command {
	case CommandResolvePools, CommandGetTokenMarket, CommandAllMethods:
		mint, err = promptMint(reader)
		if err != nil {
			return err
		}
	case CommandBatchAll:
		// No mint prompt is needed; batch-all iterates the built-in sample mint list.
	case CommandGetPool:
		pool, err = prompt(reader, "Pool address", "")
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported command %q", command)
	}

	if command == CommandAllMethods || command == CommandBatchAll {
		protocol, err = prompt(reader, "Protocol override (optional)", "")
		if err != nil {
			return err
		}
	}

	return RunCommand(ctx, runner, command, mint, pool, protocol)
}

func promptMint(reader *bufio.Reader) (string, error) {
	choice, err := prompt(reader, "Mint index from list OR full mint", "1")
	if err != nil {
		return "", err
	}
	choice = strings.TrimSpace(choice)
	if idx, convErr := strconv.Atoi(choice); convErr == nil {
		if idx >= 1 && idx <= len(SampleMints) {
			return SampleMints[idx-1], nil
		}
		return "", fmt.Errorf("index out of range: %d", idx)
	}
	return choice, nil
}

func prompt(reader *bufio.Reader, label string, defaultValue string) (string, error) {
	if defaultValue == "" {
		fmt.Printf("%s: ", label)
	} else {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}
