# EC2ASGCurl

A CLI tool to send HTTP(S) GET or POST requests to all EC2 instances in a given AWS Auto Scaling Group (ASG).

## Usage

Ensure you set your AWS env vars, be it AWS_PROFILE / AWS_ACCESS_KEY etc. 

```sh
# -h for help
./ec2asgcurl [options]
```

### Required Options

- `-asg-name`      Name of the Auto Scaling Group (required)
- `-region`        AWS region (required)

### Optional Options

- `-path`          HTTP path to call on each instance (default: `/`)
- `-port`          Port to use for the HTTP request (default: `80`)
- `-tls`           Enable TLS (use HTTPS instead of HTTP) (default: `false`)
- `-post`          File to POST as request body (if set, POST is used instead of GET)
- `-request-type`  Content-Type for the request (default: `application/json`)
- `-timeout`       HTTP request timeout (default: `3s`, example: `1.5s`, `500ms`, `2m`)
- `-headers`       Comma-separated list of headers (e.g. `key=value,key2=value2`)

## Example: GET Request

```sh
./ec2asgcurl -asg-name my-asg -region eu-west-2 -path /healthz -port 8080
```

## Example: POST Request

Suppose you have a file `payload.json`:

```json
{
  "foo": "bar"
}
```

Run:

```sh
./ec2asgcurl -asg-name my-asg -region eu-west-2 -post payload.json -request-type application/json -path /api/submit
```

## Example: Custom Headers

```sh
./ec2asgcurl -asg-name my-asg -region eu-west-2 -headers "Authorization=Bearer123,Custom-Header=Value"
```

## Example Output

```
Instance ID           IP              Launch Time               State        Resp Time       Status
-----------------------------------------------------------------------------------------------
i-0123456789abcdef0   10.0.1.23       2024-06-01T12:34:56Z      running      105.2ms         OK
 i-0fedcba9876543210   10.0.2.34       2024-06-01T12:35:12Z      running      98.7ms          OK
 i-0a1b2c3d4e5f6a7b8   10.0.3.45       2024-06-01T12:36:01Z      stopped      0s              Skipped
```

## License

Apache 2.0 
