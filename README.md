[![Ceasefire Now](https://badge.techforpalestine.org/default)](https://techforpalestine.org/learn-more)

# verifiedPetition

### What is this?
`verifiedPetition` is a simple web application that shows a petition to the browser.

When the user navigates to the website, they are presented with the petition and a form to enter their email.

The purpose behind this repository is to demonstrate to those interested in the numbers of people
signing the petition how signatures were collected. Please feel free to open issues if
you discover ways to add duplicate emails, skip captcha, or otherwise spam the service.

`verifiedPetition` is a community project associated with <a target="_blank" href="https://techforpalestine.org/">Tech for Palestine</a>.

### How does it work?
When a user asks to sign the petition, if their email belongs to one of the allowed domains in 
`cmd/verifiedPetition/data/allowed_email_domains.txt`, they receive an email through SendGrid which points to the
petition site at `/notarize?q=<encrypted-submission>`.

If the signer chooses to anonymize their email, a shasum of the email is used instead.

When the user clicks the link in the email, the server decrypts the submission and stores it in the database.

### What happens with the data?
That's up to you :) the creators of this project intend to publish the results to the relevant companies

### How to host this yourself

Feel free to fork this, change the form, and host it for your own purposes.
This particular repository was created with Palestine in mind.

#### Prerequisites

* docker
* host server with ports 22, 80, and 443 open
* SendGrid & reCaptcha (v2) API keys 

#### Procedure

* Create a file called `.env` in the root of the repository with the following contents:
```bash
PETITION_SENDGRID_API_KEY=your_sendgrid_api_key
PETITION_HOSTNAME=your.domain.com
PETITION_HTTPS=true
PETITION_ENC_KEY=your_encryption_key
PETITION_RECAPTCHA_SECRET=your_recaptcha_secret
```

* Ensure that the hostname you choose is pointed to the server you intend to host the application on.
* Ensure you've set up DNS config through sendgrid
* Ensure you've set up reCaptcha v2 through google
* Customize `index.html` and `notarize.html` to your liking
* Run `./init_server.sh user@host` to set up the server
