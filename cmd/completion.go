// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"io"
	"os"

	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion SHELL",
		Short: "Output shell completion code for the specified shell (bash or zsh)",
		Example: `  # Installing bash completion on macOS using homebrew
  ## If running Bash 3.2 included with macOS
  brew install bash-completion
  ## or, if running Bash 4.1+
  brew install bash-completion@2

  # Installing bash completion on Linux
  ## If bash-completion is not installed on Linux, please install the 'bash-completion' package
  ## via your distribution's package manager.
  ## Load the tiup completion code for bash into the current shell
  source <(tiup completion bash)
  ## Write bash completion code to a file and source if from .bash_profile
  tiup completion bash > ~/.tiup.completion.bash
  printf "
  # tiup shell completion
  source '$HOME/.tiup.completion.bash'
  " >> $HOME/.bash_profile
  source $HOME/.bash_profile

  # Load the tiup completion code for zsh[1] into the current shell
  source <(tiup completion zsh)
  # Set the tiup completion code for zsh[1] to autoload on startup
  tiup completion zsh > "${fpath[1]}/_tiup"`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				switch args[0] {
				case "bash":
					return cmd.Parent().GenBashCompletion(os.Stdout)
				case "zsh":
					return genCompletionZsh(os.Stdout, cmd.Parent())
				default:
					return errors.New("unsupported shell type " + args[0])
				}
			}

			return cmd.Help()
		},
	}

	return cmd
}

// Followed the trick https://github.com/kubernetes/kubectl/blob/master/pkg/cmd/completion/completion.go#L145.
func genCompletionZsh(out io.Writer, tiup *cobra.Command) error {
	zshHead := "#compdef tiup\n"
	_, _ = out.Write([]byte(zshHead))

	zshInitialization := `
__tiup_bash_source() {
	alias shopt=':'
	emulate -L sh
	setopt kshglob noshglob braceexpand

	source "$@"
}

__tiup_type() {
	# -t is not supported by zsh
	if [ "$1" == "-t" ]; then
		shift

		# fake Bash 4 to disable "complete -o nospace". Instead
		# "compopt +-o nospace" is used in the code to toggle trailing
		# spaces. We don't support that, but leave trailing spaces on
		# all the time
		if [ "$1" = "__tiup_compopt" ]; then
			echo builtin
			return 0
		fi
	fi
	type "$@"
}

__tiup_compgen() {
	local completions w
	completions=( $(compgen "$@") ) || return $?

	# filter by given word as prefix
	while [[ "$1" = -* && "$1" != -- ]]; do
		shift
		shift
	done
	if [[ "$1" == -- ]]; then
		shift
	fi
	for w in "${completions[@]}"; do
		if [[ "${w}" = "$1"* ]]; then
			echo "${w}"
		fi
	done
}

__tiup_compopt() {
	true # don't do anything. Not supported by bashcompinit in zsh
}

__tiup_ltrim_colon_completions()
{
	if [[ "$1" == *:* && "$COMP_WORDBREAKS" == *:* ]]; then
		# Remove colon-word prefix from COMPREPLY items
		local colon_word=${1%${1##*:}}
		local i=${#COMPREPLY[*]}
		while [[ $((--i)) -ge 0 ]]; do
			COMPREPLY[$i]=${COMPREPLY[$i]#"$colon_word"}
		done
	fi
}

__tiup_get_comp_words_by_ref() {
	cur="${COMP_WORDS[COMP_CWORD]}"
	prev="${COMP_WORDS[${COMP_CWORD}-1]}"
	words=("${COMP_WORDS[@]}")
	cword=("${COMP_CWORD[@]}")
}

__tiup_filedir() {
	# Don't need to do anything here.
	# Otherwise we will get trailing space without "compopt -o nospace"
	true
}

autoload -U +X bashcompinit && bashcompinit

# use word boundary patterns for BSD or GNU sed
LWORD='[[:<:]]'
RWORD='[[:>:]]'
if sed --help 2>&1 | grep -q 'GNU\|BusyBox'; then
	LWORD='\<'
	RWORD='\>'
fi

__tiup_convert_bash_to_zsh() {
	sed \
	-e 's/declare -F/whence -w/' \
	-e 's/_get_comp_words_by_ref "\$@"/_get_comp_words_by_ref "\$*"/' \
	-e 's/local \([a-zA-Z0-9_]*\)=/local \1; \1=/' \
	-e 's/flags+=("\(--.*\)=")/flags+=("\1"); two_word_flags+=("\1")/' \
	-e 's/must_have_one_flag+=("\(--.*\)=")/must_have_one_flag+=("\1")/' \
	-e "s/${LWORD}_filedir${RWORD}/__tiup_filedir/g" \
	-e "s/${LWORD}_get_comp_words_by_ref${RWORD}/__tiup_get_comp_words_by_ref/g" \
	-e "s/${LWORD}__ltrim_colon_completions${RWORD}/__tiup_ltrim_colon_completions/g" \
	-e "s/${LWORD}compgen${RWORD}/__tiup_compgen/g" \
	-e "s/${LWORD}compopt${RWORD}/__tiup_compopt/g" \
	-e "s/${LWORD}declare${RWORD}/builtin declare/g" \
	-e "s/\\\$(type${RWORD}/\$(__tiup_type/g" \
	<<'BASH_COMPLETION_EOF'
`
	_, _ = out.Write([]byte(zshInitialization))

	buf := new(bytes.Buffer)
	_ = tiup.GenBashCompletion(buf)
	_, _ = out.Write(buf.Bytes())

	zshTail := `
BASH_COMPLETION_EOF
}

__tiup_bash_source <(__tiup_convert_bash_to_zsh)
`
	_, _ = out.Write([]byte(zshTail))
	return nil
}
