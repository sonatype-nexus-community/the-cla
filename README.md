<!--

    Copyright (c) 2021-present Sonatype, Inc.

    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.

-->
<img src="https://github.com/sonatype-nexus-community/the-cla/raw/main/src/Header/theeecla.svg" width="100" alt="TheCla Logo"/>

<!-- Badges Section -->
[![shield_gh-workflow-test]][link_gh-workflow-test]
[![shield_license]][license_file]
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=sonatype-nexus-community_the-cla&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=sonatype-nexus-community_the-cla)

# The CLA a.k.a. `Paul Botsco`

The CLA is an app written in Golang & React for getting CLA signatures. This is just a proof of concept at time being, use at your own risk.

Inspired by [DoctoR-CLAw](https://github.com/salesforce/dr-cla).

## Deployment

The CLA is built as a Container Image and published to Docker Hub - see it [here](https://hub.docker.com/r/sonatypecommunity/the-cla).

Prior to deploying The CLA, you need to get configuration organised in GitHub itself, and have your CLA text published somewhere public
(ours is published to a public AWS S3 Bucket at `https://s3.amazonaws.com/sonatype-cla/cla.txt`, <- S3 bucket ownership info is stored in
BitWarden under "Sonatype Community" collection, "the-cla" secure note).

### GitHub Configuration

#### GitHub oAuth Application

More to come! This is where we will explain how to set up the oauth app!

see: [Creating an OAuth App](https://docs.github.com/en/developers/apps/creating-an-oauth-app) for details on how to
register `the-cla` as a new oAuth application for your account on GitHub.

For local development, you can use an `Authorization callback URL` that points to your locally running app, 
like: `http://localhost:4200/`

When you register this new oAuth app, GitHub will generate a `Client ID`.
Edit your `.env` file, setting the `REACT_APP_GITHUB_CLIENT_ID` variable to your `Client ID`. The id will be a hash-like
value like `3babf7b58e69bbd53189`. Of course your value will be different.

After completing the steps above, you also need to generate a new `Client secret` for use during authentication.
Click the `Generate a new client secret` button, and set the resulting value to the `GITHUB_CLIENT_SECRET` variable. The
id will be a hash-like value like `4a8ca4355977f4d34fe6d55ab8931fea6581024d` (a little longer than the `Client ID`).
Of course your value will be different. 
![Generate Client Secret](docs/images/generateClientSecret.png)

#### GitHub Application

You will need to create a [GitHub application](https://github.com/settings/apps/), as well. You can get to the creation page [at this link](https://github.com/settings/apps/new).

Give it a name. I like naming it `Paul Botsco`. You should like that too.

`Homepage URL` will be: http://localhost:4200/, or ngrok URL (if you are using [ngrok](ngrok.com), 
  or the actual URL of the deployed AWS service (like: https://the-cla.innovations-sandbox.sonatype.dev).

`Callback URL` will be: http://localhost:4200/, basically where you want people sent back to.

`Webhook URL` will be: http://localhost:4200/webhook-integration (strongly suggest you use ngrok for testing, it makes it way easier).
  Actually you will get an error from the Github web page if you try to use localhost 
  (`Webhook URL host localhost is not supported because it isn't reachable over the public Internet`), so gotta use ngrok.
  Here's the command I use to launch ngrok locally: `path-to-ngrok/ngrok http 4200`
  That command spins up ngrok, which show you a URL to use for connections, 
  like: `Forwarding    https://dc7d-73-194-147-232.ngrok.io -> http://localhost:4200 `
  In this case you would use `https://dc7d-73-194-147-232.ngrok.io/webhook-integration` as your `Webhook URL`.
  NOTE: Each time you launch ngrok, this endpoint will change and thus the `Webhook URL` needs to be changed.

`Webhook Secret`: this is something you can choose, out of thin air, to increase security. It's important that the 
  value you use here in GH also gets populated in your `.env` file for `GH_WEBHOOK_SECRET`, because it's how your 
  service will know how to do the super secret GH handshake. NOTE: In our case this "optional" setting is not optional!
  If forget to set this value, you will see errors like: `missing X-Hub-Signature Header`

For `Repository permissions`:

- `Administration` = Read-only
- `Contents` = Read-only
- `Issues` = Read & Write
- `Pull requests` = Read & Write
- `Commit statuses` = Read & Write

For `Organization permissions`:

- `Members` = Read-only

Under `Subscribe to events` select `Pull request`

Once you have created the app, generate and save a new private key (via `Generate a private key` button). You should save this as `the-cla.pem`, and copy it into the root of this project, it'll be noted in the next section on app environment configuration.

Set `GH_APP_ID` in `.env` to the ID for your app that was just generated! 

Move on to the next section!

#### App Environment Configuration

Configuration of `the-cla` is handled via a `.env` file in the repo (this is ignored by git by default, so you don't check in secrets) OR you can set the same environment variables using your preferred method in a Kubernetes world - see [main.tf](./main.tf) for an example as to how we use Environment Variables when deploying to Kubernetes.

A `.example.env` has been provided that looks similar to the following:

```
CLA_URL=https://s3.amazonaws.com/sonatype-cla/cla.txt
REACT_APP_COMPANY_NAME=Your company name
REACT_APP_CLA_APP_NAME=THE CLA
REACT_APP_GITHUB_CLIENT_ID=fake_ID
REACT_APP_CLA_VERSION=1.0
GITHUB_CLIENT_SECRET=fake_Secret
GH_WEBHOOK_SECRET=totallysecret
GH_APP_ID=1337
INFO_USERNAME=theInfoUsername
INFO_PASSWORD=theInfoPassword
CLA_PEM_FILE=/path/to/the-cla.pem
```

The important things to update are:

- `CLA_URL` - this is a txt file hosted somewhere that has your CLA text! We externalized this to make it easy to update, etc.
- `REACT_APP_COMPANY_NAME` - unless you want it to say `Your company name`, I would update this!
- `REACT_APP_CLA_APP_NAME` - if you don't like Toy Story references for a CLA bot, feel free to change this to whatever you want the app to say publicly
- `REACT_APP_GITHUB_CLIENT_ID` - this is the oAuth Client ID you will get from setting up your [GitHub oAuth application](#github-oauth-application)
- `GITHUB_CLIENT_SECRET` - this is the oAuth Client Secret you will get from setting up your [GitHub oAuth application](#github-oauth-application)
- `GH_WEBHOOK_SECRET` - if this isn't filled out, you won't be able to process webhooks! This is the value you set on your [GitHub App](#github-application) for an "Optional" secret (authors note, it's not optional)
- `GH_APP_ID` - this is the generated ID for the [GitHub App](#github-application) you set up!
- `SSL_MODE=disable` - this only exists to enable local development with a local database. Remove this setting for deployment to AWS.
- `INFO_USERNAME` - the username to access the "info" endpoint, e.g. to check if a particular login has signed the cla.
- `INFO_PASSWORD` - the password to access the "info" endpoint, e.g. to check if a particular login has signed the cla.
- `CLA_PEM_FILE` - Path to `the-cla.pem` (optional - defaults to just `the-cla.pem` if not defined)

Since these are all environment variables, you can just set them that way if you prefer, but it's important these variables are available at build time, as we inject these into the React code, which is honestly pretty sweet!

- `REACT_APP_COMPANY_NAME`, `REACT_APP_CLA_APP_NAME`, `REACT_APP_GITHUB_CLIENT_ID`

Additionally, to communicate with the GitHub API, you will need to have the pem file that is generated when you set up your GitHub App, in the root of this repo. All of our scripts have it named `the-cla.pem`, so if you name it that, you change nothing, and the Docker build works, etc...

#### App Installation on Repository

One more step...install the [GitHub App](https://github.com/settings/apps) you created above on a repository, so it can 
do it's thang. See [Installing GitHub Apps](https://docs.github.com/en/developers/apps/managing-github-apps/installing-github-apps).

Click the `Edit` button to edit the GitHub App. This will show a sidebar on the left that includes a 
`Install App` link. Click it, and chose an organization or account under which to install the app, and click `Install`.
Select which repositories (e.g. `all` or `some`) in which to install the app.

To verify `the-cla` is working, you can create a new Pull Request in a repository you just setup with the app.
You can view the deliveries made by the app in the `Advanced` tab (after clicking `Edit`) of [Developer Settings - GitHub Apps](https://github.com/settings/apps)
for your `Paul Botsco` GitHub App.

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for details.

## The Fine Print

Remember:

This project is part of the [Sonatype Nexus Community](https://github.com/sonatype-nexus-community) organization, which is not officially supported by Sonatype. Please review the latest pull requests, issues, and commits to understand this project's readiness for contribution and use.

* File suggestions and requests on this repo through GitHub Issues, so that the community can pitch in
* Use or contribute to this project according to your organization's policies and your own risk tolerance
* Don't file Sonatype support tickets related to this project— it won't reach the right people that way

Last but not least of all - have fun!

<!-- Links Section -->
[shield_gh-workflow-test]: https://img.shields.io/github/actions/workflow/status/sonatype-nexus-community/the-cla/ci.yml?branch=main&logo=GitHub&logoColor=white "build"
[shield_license]: https://img.shields.io/github/license/sonatype-nexus-community/the-cla?logo=open%20source%20initiative&logoColor=white "license"

[link_gh-workflow-test]: https://github.com/sonatype-nexus-community/the-cla/actions/workflows/ci.yml?query=branch%3Amain
[license_file]: https://github.com/sonatype-nexus-community/the-cla/blob/main/LICENSE
