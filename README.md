# Bunny Sync

Tool to sync local folder to [Bunny Storage](https://bunny.net/pricing/storage/) using [HTTP API](https://docs.bunny.net/reference/storage-api).

## Usage

```txt
Usage of bunnysync:
  -dry-run
    	dry run (performs no changes to remote)
  -endpoint string
    	storage endpoint (default "https://storage.bunnycdn.com")
  -password string
    	storage password
  -src string
    	path to the directory to sync
  -zone-name string
    	storage zone name
```

Example:

```bash
bunnysync \
    -src ./public \
    -zone-name $STORAGE_ZONE_NAME \
    -password $STORAGE_PASSWORD
+ blog/implementing-microsoft-rest-api-filter/index.html
+ blog/index.html
- img/me.hover.jpg
```

## Motivation

I'm trying out Bunny for my website.
Uploading files to Storage is not trivial if you do not want to drag'n'drop files using UI.
What I am talking about:

- Attempt to use some FTP (recommended approach by Bunny) GitHub Actions failed due to weird behavior - they just create a lot of nested empty folders and that's it.
- They had plans to provide S3 API, but [it is still not there and might never be](https://bunny.net/blog/whats-happening-with-s3-compatibility/).
- Using `curl` is possible but I do not want to re-upload all the files every time something is changed.
I want to upload only what was changed, in parallel.
Sounds complicated for `bash` and `curl` only solution.

That's why this tool was built.

See [Real life usage example](https://github.com/skibish/sergeykibish.com/blob/faf72c35bc77cb96ac211496fafe15a09d8b0f29/.github/workflows/deploy.yml#L43-L56).

## Test

```bash
go test -v -cover -race ./...

# If you want to update/extend sync scenario, then to "record" real responses execute the code below.
# This will update the api_fixtures.json file.
TEST_RECORD=true BUNNY_PASSWORD=$BUNNY_PASSWORD BUNNY_STORAGE_ENDPOINT=https://storage.bunnycdn.com go test -v -cover -race ./...
```
