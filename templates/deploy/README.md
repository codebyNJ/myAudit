# Deploy (Docker + Terraform)

Deploy assets myIntern scaffolds for a template app.

## Build the image

```bash
docker build -t acme-saas-api:latest -f templates/deploy/Dockerfile .
```

## Provision with Terraform (Docker provider)

Brings up mongo + valkey + the API on one network. Real cloud targets swap the
`docker` provider for AWS/GCP/etc.; the resource shape stays the same.

```bash
cd templates/deploy/terraform
terraform init
terraform apply \
  -var 'api_image=acme-saas-api:latest' \
  -var "jwt_secret=$(openssl rand -hex 32)" \
  -var "cookie_secret=$(openssl rand -hex 32)"
```

Secrets are passed as sensitive vars — never hardcoded, matching the template's
strict-env rule. Tear down with `terraform destroy`.
