# revproxyry

revproxyry is a reverse proxy with integrated Let's encrypt client
that automatically renews SSL certificates.

## Story
We needed to deploy web servers to hundreds of machines that should support 
SSL. 

Each instance had individual and pre-defined reverse proxying to static 
files and ports on the local host automatically generated from separate
meta-configuration files. We picked Nginx as our first choice. Unfortunately,
the task turned out to be quiet complex. The code to modify the Nginx 
configuration became clumsy fast. 

To provide encryption, we considered buying SSL certificates for this many 
hosts. It became soon clear that bought certificates were too costly. They
would be a pain to manage and renew automatically. No one of our users needed 
high security, the basic encryption was enough. We turned to 
[Let's encrypt](https://letsencrypt.org/). But managing the automatic renewal
of Let's encrypt's certificates proved messy as well. The provided tools,
while easy to install manually, were difficult to add to our pipeline and
lead to yet another messy part in the deployment script.

This lead us to the conclusion: there must be a simpler way. We didn't want
any of the Nginx more advanced features. Just a reverse proxy to either
a path or a microservice. And we wanted dumb-simple configuration files, 
a single flag to give us Let's encrypt's SSL 
certificates and [basic authentication](https://en.wikipedia.org/wiki/Basic_access_authentication), 
if possible. 

That's why we implemented _revproxyry_. We decided to use Go as it already
provided all the ingredients we needed: a simple handler for reverse 
proxying and an easy to use ACME client needed for the communication with
Let's encrypt.

## Related Projects

* [nginx-proxy](https://github.com/jwilder/nginx-proxy) with 
[lets encrypt companion](https://github.com/JrCs/docker-letsencrypt-nginx-proxy-companion)
provides a docker-based solution. We find the configuration of these two 
modules still a bit too complex for our taste. 

## Installation

### Pre-compiled Binaries

We provide the following pre-compiled binaries of the revproxyry:

Version|Arch|Release
---|---|---
1.0.0|Linux x64|[revproxyry-1.0.0-linux-x64.tar.gz](https://bitbucket.org/parqueryopen/revproxyry/downloads/revproxyry-1.0.0-linux-x64.tar.gz)

To install the release, just unpack it somewhere, add `bin/` directory to 
your `PATH` and you are ready to go.

### Debian Packages

We also provide a Debian package:

Version|Arch|Release
---|---|---
1.0.0|amd64|[revproxyry_1.0.0_amd64.deb](https://bitbucket.org/parqueryopen/revproxyry/downloads/revproxyry_1.0.0_amd64.deb)

For example, to download the package and install it, call:

```bash
wget https://bitbucket.org/parqueryopen/revproxyry/downloads/revproxyry_1.0.0_amd64.deb
sudo dpkg -i revproxyry_1.0.0_amd64.deb
```

### Compile From Source

Assuming you have a working Go environment, you can install the _revproxyry_
from the source by running:

```bash
go get -U bitbucket.org/parqueryopen/revproxyry
```

## Usage

You start the revproxyry by pointing it to the configuration file (see 
Section [Configuration](###Configuration)):

```bash
revproxyry --config_path /path/to/some/configuration.json
```

If you want to make it quiet, use `--quiet`:

```bash
revproxyry \
    --config_path /path/to/some/configuration.json \
    --quiet
```

To terminate _revproxyry_, send SIGTERM to the process.

You can generate the password hashes either by using 
[revproxyhashry](https://bitbucket.org/parqueryopen/revproxyhashry), 
a hashing tool developed by us with a very simple interface in mind, or a more complex Apache's 
[htpasswd](https://httpd.apache.org/docs/2.4/programs/htpasswd.html).

### Configuration

The configuration file is written in [JSON](https://en.wikipedia.org/wiki/JSON) 
format as a JSON object. 

The configuration file specifies the access and the routes of the reverse 
proxying. If you want to start straight with an example, please jump to Section 
[Example Configuration](####Example Configuration). Otherwise, here is a 
detailed list of the configuration properties:

* `domain`: specifies the domain name of the machine.

* `lets_encrypt_dir`: points to where the Let's encrypt data is stored. 

  If empty or undefined, Let's encrypt will not be used.

* `ssl_key_path`: points to the SSL key path, if you don't want to use Let's
  encrypt, but want to provide an SSL key instead. 
  
  Leave this field empty or undefined if you don't want to use SSL key. 
  
* `ssl_cert_path`: points to the SSL certificate, if you don't want to
  use Let's encrypt's certificates.
  
  Analogous to `ssl_key_path`, leave this field empty or unspecified
  if you don't want to use your own SSL certificate.
  
* `http_address`: specifies the address on which to listen to HTTP requests, 
  usually `:80`.

* `https_address`: specifies the address on which to listen to HTTPS requests,
  usually `:443`.

* `auths`: defines the authorization as a pair (user name, password hash).

  Each authorization is identified by its key in `auths` and specifies:
  
  * `username`: of the authorized user
  * `password_hash`: either Apr1 MD5 hash or bcrypt hash.
  
    You can generate hashes either with 
    [revproxyhashry](https://bitbucket.org/parqueryopen/revproxyhashry) or with Apache's 
    [htpasswd](https://httpd.apache.org/docs/2.4/programs/htpasswd.html).  
    
    If the `username` is empty, everybody is authorized.
  
* `routes`: lists the routes of the reverse proxy. 

  Each route is a JSON object which specifies:  
  
  * `auths`: the list of authorization identifiers as defined in `auths`.
  
    If `auths` is an empty list or undefined, everybody is granted access.
    
  * `target`: path to a directory, path to a file or URL.
  
  * `prefix`: path prefix of the reversed path. 
  
    Mind that the prefix is stripped from the request. 
    
    If the `target` is a path, the remainder of the requested path is
    appended to it to resolve the actual path to the directory or file
    on the disk.
    
    If the `target` is an URL, the remainder of the requested path is 
    appended to the path part of the URL.
  
If revproxyry is configured to use HTTPS, whenever the user goes to an 
HTTP URL, s/he will be automatically redirected to an HTTPS URL.


#### Example Configuration

We demonstrate here an example configuration. It instruct the reverse
proxy to use Let's encrypt and authorizes two users, `somebody` and
`somebody_else` to access different routes to local directories and 
URLs. 

The password of the user `somebody` was generated using Apache's 
[htpasswd](https://httpd.apache.org/docs/2.4/programs/htpasswd.html) and the
password of the user `somebody_else` was generated using 
[revproxyhashry](https://bitbucket.org/parqueryopen/revproxyhashry),
respectively.

```json
{
  "domain": "some-subdomain.example.com",
  "letsencrypt_dir": "/some/dir/to/lets/encrypt",
  "ssl_key_path": "",
  "ssl_cert_path": "",
  "http_address": ":80",
  "https_address": ":443",
  "auths": {
    "everybody": {
      "username": "",
      "password_hash": ""
    },
    "somebody": {
      "username": "somebody",
      "password_hash": "$apr1$TBUT11YV$MKTEAeq9GU731f4ZanSuE/"
    },
    "somebody_else": {
      "username": "somebody_else",
      "password_hash": "$2a$14$5OzUXHawpYze7RZVpkNZSe09iFd6jNLIIh/KENNfg.uLsSOfcVj2y"
    }
  },
  "routes": [
    {
      "auths": [
        "everybody"
      ],
      "target": "/some/directory",
      "prefix": "/some-public-directory/"
    },
    {
      "auths": [],
      "target": "/another/directory/file.txt",
      "prefix": "/public-file.txt"
    },
    {
      "auths": [
        "somebody"
      ],
      "target": "http://127.0.0.1:11080/",
      "prefix": "/somebody/service"
    },
    {
      "auths": [
        "somebody_else"
      ],
      "target": "http://127.0.0.1:8055/",
      "prefix": "/somebody_else/another-service"
    }
  ]
}
```

## Development

* Clone the repository beneath your `GOPATH`:

```bash
go get bitbucket.org/parqueryopen/revproxyry
```

* Change to the _revproxyry_ directory:

```bash
cd $GOPATH/src/bitbucket.org/parqueryopen/revproxyry
```

* If you want to build everything in the project:

```bash
go build ./...
```

* If you want to build and install everything to $GOPATH/bin:

```bash
go install ./...
```

* Create a pull request and send it for review `:)`

## Versioning

We follow [Semantic Versioning](http://semver.org/spec/v1.0.0.html). 
The version X.Y.Z indicates:

* X is the major version (backward-incompatible),
* Y is the minor version (backward-compatible) and
* Z is the patch version (backward-compatible bug fix).
