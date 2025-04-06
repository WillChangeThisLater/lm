documentation() {
    lynx -dump http://localhost:6060/pkg/github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types/#ConverseOutput | head -n 1200
}

builderror() {
	cat <<EOF
	\`\`\`bash
	> go build
	$(go build 2>&1)
	\`\`\`
EOF
}

project() {
cat <<EOF
I have a CLI tool (written in Go) for calling an LLM.
Here is a directory tree:

$(tree)

And here are the contents of relevant files:

$(files-to-prompt .)
EOF
}

main() {
cat <<EOF
$(project)

\`\`\`references
$(documentation)
\`\`\`
EOF
}

main
