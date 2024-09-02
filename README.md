## TODO

### Usage

#### CLI
```bash
echo "1 + 1" | go-llm
```

### Build
```bash
cd ~/go-llm # or wherever you cloned this
go build
```

### Features
- Add support for `--json-schema`
- Add support for images via `--image-urls`. Also maybe build a small CLI utility that will base64-encode
  an image provided on the CLI and produce a URL for it. Maybe something like this

  ```bash
  echo "Are the following images the same? Why or why not" | llm --image-urls "$(urlify image1.png)" "$(urlify image2.png)"
  ```

  building out `urlify` is a task. i think there are multiple approaches that could be taken here.
  previously on my work machine i used a local python server to expose the files and used ngrok
  to funnel traffic in. but that's a little brittle.

  a hacky but easy solution is to add a 'data' directory to the `urlify` tool.
  whenever `urlify` is run over file(s), it copies the file(s) to the local data directory,
  commits them to git on an `uploads` branch, then pushes the change and gets the file URLs from there.

  the "right" solution is probably to build out an entirely separate server (with domain and everything)
  and create endpoints that allow an authenticated user to add or delete files. the advantage
  of that approach is that the server to filter out requests from specific domains. so for instance,
  in this case, where we're using urlify to expose files to OpenAI, we'd have some kind of corresponding
  filter that only allows requests from the openai domain.

  but the "right" solution is also a task. in a very basic form the parts would be:

  - Build a server, `urlify-server`
    - add three endpoints for GET, POST, and DELETE of files
    - add a ping endpoint so we can do health checks
    - maybe add an endpoint for registering a tunnel domain. all this would do is store the
      name(s) of the domains that tunnel to the server, if applicable. 
    - add filtering rules so that only GET requests from specific domains are respected
      that can be something specified on startup in a list
    - add setup script to deploy the server locally

  - Build a client, `urlify-client`
    - This is a thin wrapper around `urlify-server`. The big feature it provides is that
      it can handle ngrok tunneling if/when that's needed

### Maintenance
- Add more tests
  - Make sure `--json-output` works
  - Add tests for the tokenizer
  - Test all the error cases

