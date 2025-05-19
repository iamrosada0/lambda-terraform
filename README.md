

# FleetPulse

**FleetPulse** é um sistema de telemetria para dispositivos IoT (como rastreadores veiculares), que processa dados de sensores como GPS, giroscópio e imagens. Ele utiliza serviços da AWS simulados com **LocalStack**, permitindo desenvolvimento e testes locais com serviços como **Lambda**, **SQS**, **S3**, **DynamoDB**, **Rekognition** e **Mosquitto** para MQTT.

> Desenvolvido em Go, com infraestrutura gerenciada via **Terraform** e **Serverless Framework**, este projeto é ideal para protótipos e simulações de sistemas de frotas inteligentes.

---

## 🚀 Funcionalidades

- Processamento de dados de sensores via **AWS Lambda**
- Armazenamento de dados em **DynamoDB** e imagens em **S3**
- Reconhecimento facial com **Rekognition**
- Consumo de mensagens via **SQS**
- Publicação e assinatura de tópicos MQTT com **Mosquitto**
- Infraestrutura local usando **Docker Compose** + **LocalStack**

---

## 🛠️ Tecnologias

| Categoria      | Tecnologia                       |
|---------------|----------------------------------|
| Linguagem      | Go (`provided.al2`)              |
| Backend        | AWS Lambda (via Serverless)      |
| Fila de Mensagens | AWS SQS                         |
| Banco de Dados | AWS DynamoDB                    |
| Armazenamento  | AWS S3                           |
| Reconhecimento | AWS Rekognition (simulado)       |
| MQTT           | Mosquitto                        |
| IaC            | Terraform + Serverless Framework |
| Ambiente Local | LocalStack + Docker Compose      |

---

## 📁 Estrutura do Projeto



fleet-pulse/
├── bootstrap                # Binário da Lambda
├── main.go                  # Código principal em Go
├── build.sh                 # Script para build/deploy/testes
├── terraform/               # Infraestrutura (SQS, S3, DynamoDB)
├── serverless.yml           # Lambda e Event Source Mapping
├── docker-compose.yml       # LocalStack + Mosquitto
├── mosquitto.conf           # Configuração MQTT
├── go.mod / go.sum          # Módulos Go



---

## ⚙️ Pré-requisitos

- [Go](https://golang.org/) ≥ 1.18
- [Terraform](https://www.terraform.io/downloads)
- [Node.js + npm](https://nodejs.org/) (para Serverless)
- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html)
- Docker + Docker Compose
- `zip` ou `build-lambda-zip` (Linux/WSL)
- Mosquitto CLI (opcional): `sudo apt install mosquitto-clients`

---

## 🔧 Configuração

### 1. Clone o repositório

```bash
git clone https://github.com/seu-usuario/fleet-pulse.git
cd fleet-pulse
````

### 2. Instale as dependências

```bash
go mod download
npm install  # para o Serverless Framework
```

### 3. Configure o AWS CLI

```bash
aws configure
# Access Key: test
# Secret Key: test
# Region: us-east-1
```

### 4. Inicie os serviços locais

```bash
docker-compose up -d
```

---

## 🧪 Execução

### 1. Dê permissão e execute o script de build

```bash
chmod +x build.sh
./build.sh
```

Esse script:

* Compila o código Go para `bootstrap`
* Cria o ZIP da Lambda
* Aplica os recursos do Terraform
* Implanta a Lambda via Serverless Framework
* Testa envio de mensagem SQS e consulta no DynamoDB

---

## 📤 Enviando Testes

### Enviar mensagem SQS com dados de GPS

```bash
aws --endpoint-url=http://localhost:4566 sqs send-message \
  --queue-url http://localhost:4566/000000000000/minha-fila \
  --message-body '{"type":"gps","data":{"device_id":"device123","timestamp":"2025-05-19T17:00:00Z","latitude":40.7128,"longitude":-74.0060}}'
```

### Enviar imagem base64 (simulando foto)

```bash
aws --endpoint-url=http://localhost:4566 sqs send-message \
  --queue-url http://localhost:4566/000000000000/minha-fila \
  --message-body '{"type":"photo","data":{"device_id":"device123","timestamp":"2025-05-19T17:00:00Z","image":"<base64>"}}'
```

### Consultar registros no DynamoDB

```bash
aws --endpoint-url=http://localhost:4566 dynamodb scan \
  --table-name fleet-telemetry
```

### Verificar arquivos no S3

```bash
aws --endpoint-url=http://localhost:4566 s3 ls s3://my-test-bucket
```

---

## 📡 Testes MQTT (opcional)

```bash
# Publica
mosquitto_pub -h localhost -p 1883 -t test/topic -m "Mensagem MQTT"

# Assina
mosquitto_sub -h localhost -p 1883 -t test/topic
```

---

## 📌 Notas Importantes

* O Serverless cria automaticamente o **Event Source Mapping** entre SQS e a Lambda.
  Verifique com:

```bash
aws --endpoint-url=http://localhost:4566 lambda list-event-source-mappings
```

* Para evitar o erro `InvalidClientTokenId` no Terraform, use no `main.tf`:

```hcl
provider "aws" {
  skip_requesting_account_id = true
  access_key = "test"
  secret_key = "test"
  region     = "us-east-1"
  endpoints {
    dynamodb = "http://localhost:4566"
    sqs      = "http://localhost:4566"
    s3       = "http://localhost:4566"
  }
}
```

---

## 🧭 Próximos Passos

* [ ] Conectar MQTT → Lambda via bridge com SQS
* [ ] Adicionar filtros e índices no DynamoDB
* [ ] Migrar runtime para `provided.al2023`
* [ ] Adicionar testes unitários e mocks de Rekognition
* [ ] Adicionar suporte a múltiplos dispositivos e usuários

---

## 🤝 Contribuição

Contribuições são bem-vindas! Abra uma **issue** ou envie um **pull request**. Sugestões, melhorias e correções são sempre apreciadas.

---

## 📄 Licença

Distribuído sob a licença MIT. Veja `LICENSE` para mais informações.

---

> Projeto desenvolvido por [Luis de Água Rosada](https://github.com/iamrosada0) — 2025 🚚📡


