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

#### Unstrucured JSON output

```bash
URL="https://content.r9cdn.net/rimg/dimg/9a/61/3dc6f5bb-city-25999-16ea40716ab.jpg?width=1200&height=630&xhint=2045&yhint=2190&crop=true"
echo "How many camels are in this picture?" | lm --imageURLs "$URL" --json-output  # returns something like {"camels": 3}
```

#### Structured JSON output
```bash
cat <<'EOF' >/tmp/schema.json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "sentiment": {
      "type": "string",
      "enum": [
        "great",
        "good",
        "neutral",
        "bad",
        "terrible"
      ]
    },
    "reason": {
      "type": "string"
    }
  },
  "required": [
    "sentiment",
    "reason"
  ],
  "additionalProperties": false
}
EOF

cat /tmp/schema.json | jq # make sure the schema JSON is valid
echo "Rate the following review: 'McDonalds is truly a gem of the Gary, Indiana community. Wait times are 1 hour+, but are compensated with a delightfully soggy and slimy burger. The whole joint gives that nice 'haunted house' vibe that everyone so associated with quality resturaunts'" | lm --json-schema-file /tmp/schema.json --model gpt-4o-mini
```

#### Site Input (experimental, uses gowitness to screenshot sites which can be flaky. you need chrome installed + potentially other setup for this to work)

```bash
echo "What does this author think about the future of neural networks? Give specifics on what he thinks neural networks will look like 30 years from now" | lm --sites "http://karpathy.github.io/2022/03/14/lecun1989/"
```
