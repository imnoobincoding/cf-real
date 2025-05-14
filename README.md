# Traefik Cloudflare Real IP Plugin

<p align="center">
  <a href="https://git.io/typing-svg"><img src="https://readme-typing-svg.demolab.com?font=Fira+Code&weight=600&size=28&pause=1000&color=F77F00&width=435&lines=CLoudflare+Real+IP+Plugin;Get+the+True+Client+IP+in+Traefik" alt="Typing SVG" /></a>
</p>

## Disclaimer & Original Work

This project is a fork of the original work by **vincentinttsh** ([vincentinttsh/cloudflareip](https://github.com/vincentinttsh/cloudflareip)).
The present version builds upon this foundation and includes minimal extensions, such as explicitly setting the `X-Forwarded-For` header and providing more detailed documentation for configuration options.

A big thank you to vincentinttsh for the original development!

---

When Traefik is operated behind a reverse proxy like Cloudflare (or another load balancer/firewall), Traefik, by default, only sees the IP address of this intermediate proxy as `RemoteAddr`. The actual IP address of the external client is thereby lost for the downstream applications.

This Traefik plugin solves this issue by reading the real client IP information from the `Cf-Connecting-IP` header (provided by Cloudflare) and using it to overwrite the standard `X-Forwarded-For` and `X-Real-IP` headers. This only occurs if the request originates from a trusted IP address (configurable via `trustip`).

This allows your backend services to use the correct client IP address for logging, analytics, rate-limiting, or other purposes.

## How it Works

1.  The plugin checks the `RemoteAddr` of the incoming request (i.e., the IP address of the direct peer sending the request to Traefik).
2.  If this `RemoteAddr` is included in the list of `trustip` (trusted IPs):
    a.  The plugin reads the value of the `Cf-Connecting-IP` header. This header is set by Cloudflare and contains the visitor's original IP address.
    b.  If the `Cf-Connecting-IP` header contains a value, the plugin sets this value as the new value for the `X-Forwarded-For` and `X-Real-IP` headers.
3.  If the `RemoteAddr` is not trusted or if the `Cf-Connecting-IP` header is empty, the plugin makes no changes to the `X-Forwarded-For` or `X-Real-IP` headers.

## Key Features

* Determines the real client IP for requests coming through Cloudflare (or similar proxies that set a corresponding header).
* Sets the standard `X-Forwarded-For` and `X-Real-IP` headers for downstream applications.
* Configurable list of `trustip` (trusted IPs) to ensure that the `Cf-Connecting-IP` header is only considered from trusted sources.
* Easy integration with Traefik Proxy.

## Configuration

### Prerequisites

* Traefik v2.x or v3.x
* Your Traefik setup receives traffic from Cloudflare (or another proxy that sets the `Cf-Connecting-IP` header, or one you can customize to use a similar header that the plugin should read).
* You must know the IP addresses of the last proxy *before* Traefik (e.g., Cloudflare IPs, the IPs of your firewall, or your load balancer) to configure them as `trustip`.

### Plugin Configuration

The only configuration option for this plugin is `trustip`:

| Setting   | Type       | Required | Description                                                                                                                                                                                                                            |
| :-------- | :--------- | :------- | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `trustip` | `[]string` | Yes      | A list of IP addresses or CIDR ranges. The `Cf-Connecting-IP` header will only be used as the source for the client IP if the request originates from one of these IPs. **These must be the IPs of the proxy that communicates directly with Traefik.** |

### Static Configuration (traefik.yml or command-line arguments)

You must declare the plugin in Traefik's static configuration.

```yaml
# traefik.yml

# Optional: For Traefik Pilot (if used)
# pilot:
#   token: YOUR_TRAEFIK_PILOT_TOKEN

experimental:
  plugins:
    # Give your plugin a name, e.g., "cloudflareRealIp"
    cloudflareRealIp:
      # NOTE: Adjust modulename and version to your actual Go module and version
      # If you are using this fork: [github.com/YOUR_USERNAME/YOUR_FORK_NAME](https://github.com/YOUR_USERNAME/YOUR_FORK_NAME)
      modulename: [github.com/imnoobincoding/cf-real](https://github.com/imnoobincoding/cf-real)
      version: v1.0.0 # Example: v0.1.0 or a specific commit SHA
```