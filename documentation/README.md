Claro! Vou transformar os comandos que você executou em um README explicando o que cada um faz no contexto do AWS Lambda e SQS usando LocalStack e AWS CLI.

---

# README: Como trabalhar com AWS Lambda e SQS usando LocalStack

Este tutorial mostra como listar, criar e associar fontes de eventos no AWS Lambda, enviar mensagens para SQS e monitorar logs, tudo rodando localmente com LocalStack.

---

## 1. Entrando no ambiente Ubuntu pelo WSL

```bash
wsl -d Ubuntu
```

* Abre o terminal do Ubuntu dentro do Windows usando o WSL (Windows Subsystem for Linux).

---

## 2. Listar as associações de fonte de eventos para uma função Lambda

```bash
aws --endpoint-url=http://localhost:4566 --region us-east-2 lambda list-event-source-mappings --function-name minha-funcao
```

* Lista todos os mapeamentos de fontes de eventos (por exemplo, filas SQS) associados à função Lambda chamada `minha-funcao`.
* No momento, não há associações (`EventSourceMappings` vazio).

---

## 3. Obter informações da função Lambda

```bash
aws --endpoint-url=http://localhost:4566 --region us-east-2 lambda get-function --function-name minha-funcao
```

* Exibe detalhes completos da função Lambda `minha-funcao` como runtime, tamanho do código, timeout, memória, estado, etc.

---

## 4. Obter ARN da fila SQS

```bash
aws --endpoint-url=http://localhost:4566 --region us-east-2 sqs get-queue-attributes --queue-url http://localhost:4566/000000000000/minha-fila --attribute-names QueueArn --query 'Attributes.QueueArn' --output text
```

* Consulta o atributo `QueueArn` da fila SQS chamada `minha-fila`.
* O ARN é necessário para associar a fila à função Lambda.

---

## 5. Criar associação entre fila SQS e função Lambda (EVENT SOURCE MAPPING)

```bash
aws --endpoint-url=http://localhost:4566 --region us-east-2 lambda create-event-source-mapping --function-name minha-funcao --batch-size 1 --event-source-arn arn:aws:sqs:us-east-2:000000000000:minha-fila
```

* Cria o mapeamento para que a função Lambda `minha-funcao` seja acionada por mensagens da fila `minha-fila`.
* `batch-size 1` significa processar uma mensagem por invocação.

---

## 6. Confirmar associação criada

```bash
aws --endpoint-url=http://localhost:4566 --region us-east-2 lambda list-event-source-mappings --function-name minha-funcao
```

* Lista novamente as associações para confirmar que a fila está vinculada à função Lambda.

---

## 7. Enviar mensagem para a fila SQS

```bash
aws --endpoint-url=http://localhost:4566 --region us-east-2 sqs send-message --queue-url http://localhost:4566/000000000000/minha-fila --message-body '{"id":"123","conteudo":"Mensagem de teste"}'
```

* Envia uma mensagem JSON para a fila `minha-fila`.
* Essa mensagem acionará a execução da função Lambda associada.

---

## 8. Invocar a função Lambda manualmente com payload de arquivo (com erro)

```bash
aws --endpoint-url=http://localhost:4566 lambda invoke --function-name minha-funcao --payload file://input.json output.txt
```

* Tenta invocar a função Lambda passando um arquivo `input.json` como payload.
* Erro: arquivo `input.json` não encontrado.

---

## 9. Verificar logs da função Lambda (com erro)

```bash
aws --endpoint-url=http://localhost:4566 logs filter-log-events --log-group-name /aws/lambda/minha-funcao
```

* Tenta buscar logs da função Lambda no CloudWatch local.
* Erro: grupo de logs não existe ainda.

---

## 10. Criar grupo de logs manualmente

```bash
aws --endpoint-url=http://localhost:4566 logs create-log-group --log-group-name /aws/lambda/minha-funcao
```

* Cria manualmente o grupo de logs para a função Lambda para permitir captura e visualização dos logs.

---

## 11. Invocar Lambda manualmente com payload JSON

```bash
aws --endpoint-url=http://localhost:4566 --region us-east-2 lambda invoke --function-name minha-funcao --payload '{"test": "Manual invocation"}' response.json
```

* Invoca a função Lambda com um payload JSON direto na linha de comando.
* Atenção: erros podem ocorrer se o JSON não estiver codificado em UTF-8 corretamente.

---

## 12. Visualizar logs em tempo real

```bash
aws --endpoint-url=http://localhost:4566 logs tail /aws/lambda/minha-funcao --since 1m --follow
```

* Exibe os logs da função Lambda em tempo real, mostrando eventos dos últimos 1 minuto e atualizando continuamente.

---

# Observações importantes

* Use sempre `--endpoint-url=http://localhost:4566` para direcionar as chamadas ao LocalStack.
* A região padrão usada foi `us-east-2`.
* Certifique-se que o arquivo de payload JSON exista e esteja no formato UTF-8 para evitar erros.
* O grupo de logs pode precisar ser criado manualmente no LocalStack para que os logs funcionem.
* A associação de evento entre SQS e Lambda é essencial para que a função seja acionada automaticamente ao receber mensagens.

---



```bash
# Acompanhar os logs da função Lambda (últimos 30 minutos, modo follow)
aws --endpoint-url=http://localhost:4566 logs tail /aws/lambda/minha-funcao --since 30m --follow

# Receber mensagem da fila SQS
aws --endpoint-url=http://localhost:4566 sqs receive-message --queue-url http://localhost:4566/000000000000/minha-fila

# Listar event source mappings associados à função Lambda
aws --endpoint-url=http://localhost:4566 lambda list-event-source-mappings --function-name minha-funcao

# Enviar mensagem para a fila SQS
aws --endpoint-url=http://localhost:4566 --region us-east-2 sqs send-message --queue-url http://localhost:4566/000000000000/minha-fila --message-body '{"id":"78966","conteudo":"mmmmOutra xxxxmensagem"}'

# Acompanhar os logs da função Lambda (último 1 minuto, modo follow)
aws --endpoint-url=http://localhost:4566 logs tail /aws/lambda/minha-funcao --since 1m --follow

# Para fazer scan na tabela

aws --endpoint-url=http://localhost:4566 dynamodb scan --table-name Mensagens --region us-east-1

# Permitir variáveis do direnv (caso esteja usando)
direnv allow
```

---

