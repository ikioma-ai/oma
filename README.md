# Oma

Oma is a platform for installing and managing Docker-based web applications.
Its goal is to make self-hosting applications as simple as possible.

As well as simplifying the initial setup, Oma also provides automatic updates, backups, and system information.
It has a TUI interface with a dashboard for monitoring and operating your applications, as well as CLI commands for common operations should you (or your AI agent) prefer that.

Oma runs on Linux and macOS, and can be used to run applications on a variety of hardware: a physical server, a cloud VPS, a Raspberry Pi, or your laptop are all suitable.

Oma comes with a small built-in application catalog, but you can use it to install any compatible Docker image as well.

## Installing

Install Oma with the hosted installer:

```sh
curl -fsSL https://get.oma.ikioma.ai | sh
```

The installer verifies the release signature and checksum, installs Oma, installs the background service, and starts the first-run flow.
On Linux it can install Docker when needed; macOS requires Docker Desktop.

For a controller that only manages remote targets, skip the local Docker requirement:

```sh
curl -fsSL https://get.oma.ikioma.ai | OMA_LOCAL_DOCKER=false sh
```

Then add a target with `oma target add` and `oma target setup`, and run `oma --target <name> list`.

### Building from source

You can also build Oma from source. You will need Go 1.27 or later and Docker.

```sh
make build
sudo install -m 755 bin/oma /usr/local/bin/oma
sudo oma background install
oma
```

The `oma background install` command registers the background service that handles automatic updates and backups.
Run `oma` to install an application, or `oma --help` to see the available commands.

Release and rollback details are in [`docs/release/checklist.md`](docs/release/checklist.md). Oma starts clean; migration from ONCE is not supported.

Note that if you need to use `sudo` when running Docker commands (for example, if your user is not in the `docker` group) then you'll also need to use `sudo` whenever you run `oma`.

## Using Oma to install and configure applications

On first run, Oma will prompt you to choose an application to install.
You can pick from the built-in applications, or enter the path to a Docker image.

When entering an image path you can pick any application that works with Oma (more on this below), so you can run additional applications or even your own custom forks of the built-in ones.

You'll need to enter a hostname for the application.
If you're installing onto a machine on the public Internet (like a cloud VPS), and you have a domain name, then you can use that.
It's a good idea to use a subdomain so that you can run multiple applications on the same domain.

For example, if you own `example.com` you might install Writebook to `books.example.com`.

Whatever hostname you choose, make sure you have a DNS entry for it which points to the machine you're installing to.
The details of how to do this will vary depending on which provider you use for your DNS, but generally you should have access to an admin interface where you can set up an `A` record with the hostname you've chosen, pointed to the IP address of your machine.
If you plan to install many applications on the same machine you could use a wildcard DNS entry for this.

> [!TIP]
> One tip if you're using Cloudflare: by default, DNS entries on Cloudflare will have the "proxy" option enabled, which means that traffic will pass through Cloudflare before reaching your server.
> Oma works well in this setup provided you have SSL enabled end-to-end.
> So just be sure your Cloudflare SSL mode is set to "Strict (full)" if you're using its proxy option.

Once you've picked your application and entered the hostname, the rest is automatic.
Oma will fetch, install, and boot the application, and then take you to the dashboard screen where you can monitor it.

### Changing application settings

There are various extra settings you can change on your applications.
When you have an application selected on the dashboard, press `s` to view the settings menu, and choose an item from the menu to open that screen.
From there you can set up a location for automatic backups, update your hostname, switch to using your own fork's image, set up an email provider and more.

You can also use the action menu, `a` to start and stop applications, or remove them completely.

### Customizing the `oma` appearance

On first run, `oma` shows an animated logo and background.
If you find this distracting, or it causes problems in your environment, you can suppress it with `OMA_REDUCED_MOTION=true`:

```sh
OMA_REDUCED_MOTION=true oma
```

## Making an Oma-compatible application

Fundamentally, Oma works with any web application that:

- Is packaged as a Docker container
- Serves HTTP on port 80
- Has a healthcheck endpoint at `/up` that returns success
- Keeps its persistent data in `/storage`

Any application that does these things should work with Oma.

However, beyond this bare minimum, there are some additional scripts and environment variables that allow for better integration with the platform:

### Storage paths

Oma will mount a persistent volume into `/storage`.

For compatibility with standard Rails applications, it also mounts the same volume into `/rails/storage`.
That means Rails applications built with the default Dockerfile should work just fine.

Data in this volume persists across restarts, and is also the data that will be included in backups.

### Hook scripts

Oma uses a few optional hook scripts as integration points.
Currently supported hooks are:

- `/hooks/pre-backup` -- Because Oma can't assume that it's safe to backup the files of an application while they are in use, it will try to call this hook before starting a backup so the application can do anything it needs to generate a "safe" copy of the data.
If this script exists, and returns success, Oma assumes it's now safe to copy the files into the backup.
If it doesn't exist, or returns an error, Oma will pause the application's container while it copies the file.
This means backups are always consistent in either case, but the hook gives the app a way to avoid the paused container introducing latency to in-flight requests while the backup runs.
An example of using `pre-backup` on a SQLite-based application would be to use SQLite's online backup feature to take a safe consistent copy of the database.

- `/hooks/post-restore` -- The inverse of `pre-backup`, `post-restore` will be called after restoring the data from a backup, but before booting the application.
If an application needs to do any cleanup, such as move or rename files generated during `pre-backup`, it can do it in this hook.

### Environment variables

Many of the configuration settings that you can do in the Oma UI make their way into the application in the form of environment variables.
These include:

- `SECRET_KEY_BASE` -- A unique identifier, generated at installation time, and kept for the life of the applications.
By convention, Rails applications use this as the base for cryptographic signing.
- `DISABLE_SSL` -- This will be set to `true` if the app is running without SSL.
This can be useful for applications that generate redirects or URLs that would otherwise assume SSL is being used.
- `BASE_URL` -- The public URL of the application, built from its hostname, SSL setting, and proxy port (e.g. `https://app.example.com`).
Applications can use this to generate links outside of a web request, such as in emails.
- `VAPID_PUBLIC_KEY`/`VAPID_PRIVATE_KEY` -- For applications that use WebPush, unique VAPID credentials are automatically generated and passed in these variables.
- `SMTP_ADDRESS`/`SMTP_PORT`/`SMTP_USERNAME`/`SMTP_PASSWORD`/`MAILER_FROM_ADDRESS` -- The values from the Email Settings screen are passed in these.
- `NUM_CPUS` -- If an application is restricted with a CPU quota, this variable will contain the number of CPUs it has been allowed to use.
An application can use this to vary the number of worker processes it spawns to an appropriate number for that quota.
