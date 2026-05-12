# cruise-schedule-bot

Lambda function en Go que cada día lee las escalas de cruceros del [Puerto de A Coruña](https://www.puertocoruna.com/escalas-de-cruceros) y envía los barcos del día (con hora de llegada y salida) a un chat de Telegram.

## Requisitos

| Herramienta | Versión mínima |
|-------------|---------------|
| Go          | 1.21          |
| AWS CLI     | 2.x           |
| AWS SAM CLI | 1.x (opcional)|
| zip         | cualquiera    |

## Configuración de Telegram

1. Habla con [@BotFather](https://t.me/BotFather) en Telegram → `/newbot` → guarda el **token**.
2. Añade el bot al grupo/canal donde quieras recibir los mensajes.
3. Obtén el **chat ID**:
   - Para un grupo: envía un mensaje, luego llama a  
     `https://api.telegram.org/bot<TOKEN>/getUpdates`  
     y busca el campo `"id"` dentro de `"chat"`.
   - Para un canal: usa `@channelname` (con la arroba) como chat ID.

## Build

```bash
make zip          # Compila y empaqueta → function.zip
```

El binario se compila estáticamente para Amazon Linux 2023 (`GOOS=linux GOARCH=amd64 CGO_ENABLED=0`).

## Despliegue

### Opción A — AWS SAM (recomendado)

```bash
sam deploy --guided \
  --template-file template.yaml \
  --stack-name cruise-schedule-bot \
  --parameter-overrides \
      TelegramBotToken=<TOKEN> \
      TelegramChatId=<CHAT_ID>
```

### Opción B — AWS CLI manual

```bash
# 1. Crear la función (primera vez)
aws lambda create-function \
  --function-name cruise-schedule-bot \
  --runtime provided.al2023 \
  --role arn:aws:iam::<ACCOUNT_ID>:role/<ROLE_NAME> \
  --handler bootstrap \
  --zip-file fileb://function.zip \
  --environment "Variables={TELEGRAM_BOT_TOKEN=<TOKEN>,TELEGRAM_CHAT_ID=<CHAT_ID>}" \
  --timeout 30 \
  --memory-size 128

# 2. Crear el EventBridge trigger (07:00 UTC diario)
aws events put-rule \
  --name cruise-schedule-daily \
  --schedule-expression "cron(0 7 * * ? *)" \
  --state ENABLED

aws lambda add-permission \
  --function-name cruise-schedule-bot \
  --statement-id AllowEventBridge \
  --action lambda:InvokeFunction \
  --principal events.amazonaws.com \
  --source-arn arn:aws:events:<REGION>:<ACCOUNT_ID>:rule/cruise-schedule-daily

aws events put-targets \
  --rule cruise-schedule-daily \
  --targets "Id=1,Arn=arn:aws:lambda:<REGION>:<ACCOUNT_ID>:function:cruise-schedule-bot"

# 3. Actualizar el código (siguientes veces)
make deploy LAMBDA_FUNCTION_NAME=cruise-schedule-bot
```

## Variables de entorno

| Variable              | Descripción                              |
|-----------------------|------------------------------------------|
| `TELEGRAM_BOT_TOKEN`  | Token del bot de Telegram                |
| `TELEGRAM_CHAT_ID`    | ID del chat/grupo/canal destino          |

## Tests

```bash
go test ./...
```

## Horario de ejecución

La regla de EventBridge está configurada para `cron(0 7 * * ? *)`:
- **07:00 UTC** → 08:00 en invierno (CET) / 09:00 en verano (CEST)

Ajusta la hora en `template.yaml` si prefieres recibirlo antes.

## Adaptar los selectores HTML

Si el puerto cambia el diseño de su web, edita `parseCruises()` en `main.go`.  
La función busca filas `<tr>` dentro de cualquier `<table>` con esta estructura de columnas:

```
[0] Fecha  [1] Buque  [2] Procedencia  [3] Llegada  [4] Salida  [5] Compañía
```

Si la estructura es diferente, cambia los índices `cells.Eq(N)` según corresponda.
