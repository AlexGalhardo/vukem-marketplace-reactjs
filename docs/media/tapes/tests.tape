# Records docs/media/tests.gif from the repo root:
#   docker build -t vukem-mkt-vhs docs/media/tapes
#   docker run --rm -v vmkt-mod:/root/go -v vmkt-build:/root/.cache/go-build -v "$PWD:/src:ro" -v "$PWD/docs/media:/out" vukem-mkt-vhs docs/media/tapes/tests.tape
# (the container sees the repo at /src and the output directory at /out)
Output "/out/tests.gif"

Set Shell bash
Set Theme "Catppuccin Mocha"
Set FontSize 16
Set Width 1000
Set Height 620
Set Padding 20
Set TypingSpeed 55ms
Set PlaybackSpeed 2

Hide
Type "mkdir /work && tar -C /src --exclude=node_modules --exclude=.git -cf - . | tar -C /work -xf - && cd /work && set -o pipefail && export PS1='\[\e[32m\]\w\[\e[0m\] $ ' && go test -count=1 ./... >/dev/null 2>&1; clear; echo REA''DY" Enter
Wait+Screen@900s /READY/
Type "clear" Enter
Sleep 300ms
Show

Type "go test -count=1 ./... | grep -v 'no test files' && echo 'all tests passed'" Sleep 400ms Enter
Sleep 60s
Sleep 4s
