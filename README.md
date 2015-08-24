# instagger

## How to launch

1. Obtain instagram `access_token`, place it to `/instagger/access_token` file on Linux host.
2. Write comma-separated hashtags to `/instagger/hashtags`. (e.g. `#foo,#bar`)
3. Build this image: `docker build -t instagger .`
4. Run image:

```sh
docker run -d --restart=always --name=instagger_agent \
	-e ACCESS_TOKEN="`cat /instagger/access_token`" \
	-e HASHTAGS="`cat /instagger/hashtags`" \
	instagger
```

Check if it is running:

    docker logs -f instagger_agent

