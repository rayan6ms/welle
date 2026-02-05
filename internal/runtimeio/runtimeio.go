package runtimeio

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

var (
	ErrInputUnavailable   = errors.New("input is not available in non-interactive mode")
	ErrGetpassUnavailable = errors.New("getpass is not available in non-interactive mode")
)

func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func Input(prompt string) (string, error) {
	if !IsInteractive() {
		return "", ErrInputUnavailable
	}
	if prompt != "" {
		_, _ = fmt.Fprint(os.Stdout, prompt)
	}
	line, err := readLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", ErrInputUnavailable
		}
		return "", err
	}
	return line, nil
}

func GetPass(prompt string) (string, error) {
	if !IsInteractive() {
		return "", ErrGetpassUnavailable
	}
	if prompt != "" {
		_, _ = fmt.Fprint(os.Stdout, prompt)
	}
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		if b, err := term.ReadPassword(fd); err == nil {
			return string(b), nil
		}
	}
	line, err := readLine()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", ErrGetpassUnavailable
		}
		return "", err
	}
	return line, nil
}

func readLine() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
