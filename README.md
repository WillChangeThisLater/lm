## TODO

### Build

```bash
git clone https://github.com/WillChangeThisLater/lm
cd lm
go build
ln -s $(pwd)/lm /usr/local/bin/lm # link into PATH
```

### Usage

#### Basic Usage

```bash
echo "1 + 1" | lm
```

#### Image Input (from internet URLs)

```bash
colosseumURL="https://upload.wikimedia.org/wikipedia/commons/thumb/d/de/Colosseo_2020.jpg/800px-Colosseo_2020.jpg"
pyramidURL="https://upload.wikimedia.org/wikipedia/commons/e/e3/Kheops-Pyramid.jpg"
echo "Which of these buildings is older?" | lm --imageURLs "$colosseumURL,$pyramidURL"
```

#### Image Input (from local files)

```bash
curl -o /tmp/colosseum.jpg https://upload.wikimedia.org/wikipedia/commons/thumb/d/de/Colosseo_2020.jpg/800px-Colosseo_2020.jpg
curl -o /tmp/pyramid.jpg https://upload.wikimedia.org/wikipedia/commons/e/e3/Kheops-Pyramid.jpg
echo "Which of these buildings is older?" | lm --imageFiles "/tmp/colosseum.jpg,/tmp/pyramid.jpg"
```

#### Screenshot

```bash
echo "Focus on the screenshot with YouTube open. How far am I through the video?" | lm --screenshot
```

#### Site Input (experimental, works sometimes)

```bash
echo "What does this author think about the future of neural networks? Give specifics on what he thinks neural networks will look like 30 years from now" | lm --sites "http://karpathy.github.io/2022/03/14/lecun1989/"
```

#### Run against local models

```bash
llama-server -m deepseek.gguf                       # in one terminal window
echo "hello world" | lm --model local-deepseek-7b
```

### Prompting
One pattern I find myself falling into a lot is using bash to generate prompt templates for my projects.
When I build these prompts, I'll often use lynx (terminal based web browser) to get the contents of a page
using `lynx -dump <url>`. The formatting usually needs some improvements, so I normally run the
contents from the dump through an LLM to be sanitized and maybe shortened. For instance:

```bash
references() {
cat <<EOF
  chroma:
  --------------------
  $(lynx -dump https://github.com/chroma-core/chroma | lm --prompt "this document contains very useful information. however, it also contains irrelevant links, styling, etc. repeat the core part of the documents verbatim, but leave out parts that would be irrelevant to someone using the documentation for development or coding purposes" --cache)

  aws embedding model:
  --------------------
  $(lynx -dump https://docs.aws.amazon.com/bedrock/latest/userguide/titan-embedding-models.html | lm --prompt "this document contains very useful information. however, it also contains irrelevant links, styling, etc. repeat the core part of the documents verbatim, but leave out parts that would be irrelevant to someone using the documentation for development or coding purposes. Remove the country list as well." --cache)

  aws nova multimodal API call:
  -----------------------------
  $(lynx -dump https://docs.aws.amazon.com/nova/latest/userguide/modalities-image-examples.html | lm --prompt "consider only the InvokeModel example. improve the formatting of the python code. don't explain your work - just output the python code with the improved formatting" --cache)
EOF
}

main() {
cat <<EOF
Build a simple RAG tool.

Use the following:
$(references)
EOF
}

main
```

Caching is the killer feature here - if you run the prompt again, `lm` will remember that it has seen the document summarization
prompt before, and just return the contents directly instead of calling the LLM.

### Misc

#### Project prompt
Good prompt for getting high level info about a project. 
`files-to-prompt` comes from Simon Willison

```bash
project() {
cat <<EOF
General project information:

Directory tree:

$(tree)

File contents:

$(files-to-prompt .)
EOF
}
```

#### Heredocs
```bash
cat <<EOF

\`\`\`bash
> ls
$(ls)
\`\`\`
EOF
```

```bash
cat <<'EOF'
heredoc literal

special characters can be used here

$^#!

$(ls) won't enter a subshell or anything
EOF
```


### Wishlist
#### `lm`
- some way to generate one/few shot examples from a prompt? but only if it actually improves accuracy

#### Common prompt library
`showProject()` for showing project details (tree, repocat, etc.)
`showError()` for showing error message
`bashCommand()` for running bash stuff - this would take care of bash formatting and I/O redirection
`showSite()` for scraping a site (w/ optional LLM summarization/filtering/caching)

#### Heredoc indentation
I wish the indentation for heredocs was better. Having to throw everything out on the margin to the left
causes me pain
