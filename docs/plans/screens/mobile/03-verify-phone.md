# Tela: Verificacao Inicial (OTP)

**App:** Mobile Validador
**Prioridade:** MVP
**Plataformas:** iOS, Android

---

## Objetivo

Confirmar que o numero informado pertence ao usuario atraves de um codigo OTP enviado por SMS.
Esta verificacao ocorre **apenas uma vez**, no registro do dispositivo.

---

## Layout

```
┌─────────────────────────────┐
│ ←                           │
│                             │
│  Verificacao                │
│                             │
│  Enviamos um codigo para    │
│  +55 11 9****-9999          │
│                             │
│                             │
│     ┌──┐ ┌──┐ ┌──┐ ┌──┐    │
│     │  │ │  │ │  │ │  │    │
│     └──┘ └──┘ └──┘ └──┘    │
│                             │
│     Codigo de 4 digitos     │
│                             │
│                             │
│  Nao recebeu?               │
│  Reenviar em 0:45           │
│                             │
│                             │
│  ┌───────────────────────┐  │
│  │   Verificar            │  │
│  └───────────────────────┘  │
│                             │
└─────────────────────────────┘
```

---

## Estados

### 1. Aguardando Codigo
- Campos de input vazios, cursor no primeiro campo
- Timer de reenvio contando (60s)
- Botao "Verificar" desabilitado

### 2. Codigo Inserido
- 4 digitos preenchidos
- Botao "Verificar" habilitado

### 3. Verificando
- Loading spinner no botao
- Campos desabilitados

### 4. Erro - Codigo Invalido
```
┌──┐ ┌──┐ ┌──┐ ┌──┐
│ 1│ │ 2│ │ 3│ │ 4│  ← borda vermelha
└──┘ └──┘ └──┘ └──┘
  Codigo incorreto.
  Tente novamente.
```

### 5. Reenvio Disponivel
- Link "Reenviar codigo" ativo (apos timer zerar)

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Digitar numero | Auto-avanca para proximo campo |
| Colar codigo | Preenche todos os campos |
| Tap "Verificar" (correto) | Registra dispositivo → Navega para Home |
| Tap "Verificar" (incorreto) | Exibe erro, limpa campos |
| Tap "Reenviar" | Novo SMS, reinicia timer |
| Tap "←" | Volta para Registro de Telefone |
| Auto-fill SMS (Android) | Preenche automaticamente via SMS Retriever API |

---

## API Calls

### Verificar Codigo
```
POST /internal/devices/verify/confirm
{
  "verification_id": "uuid",
  "code": "1234"
}

Response 200:
{
  "device_id": "uuid",
  "phone_number": "+5511999999999"
}
```

### Registrar Dispositivo (apos verificacao)
```
POST /internal/devices
{
  "phone_number": "+5511999999999",
  "device_token": "fcm_token_here",
  "platform": "android"
}

Response 201:
{
  "id": "uuid",
  "status": "active"
}
```

### Reenviar Codigo
```
POST /internal/devices/verify/resend
{
  "verification_id": "uuid"
}
```

---

## Validacoes

- Codigo: exatamente 4 digitos numericos
- Maximo 3 tentativas de codigo incorreto por verificacao
- Maximo 3 reenvios por numero por hora
- Codigo expira em 5 minutos

---

## Tratamento de Erros

| Erro | Mensagem |
|------|----------|
| Codigo incorreto | "Codigo incorreto. Tente novamente." |
| Codigo expirado | "Codigo expirado. Solicite um novo." |
| Tentativas esgotadas | "Muitas tentativas. Solicite um novo codigo." |
| Reenvio rate limit | "Aguarde antes de solicitar novo codigo." |

---

## Notas Tecnicas

- Auto-read SMS no Android (SMS Retriever API)
- iOS: suporte a auto-fill do teclado
- Salvar `device_id` e `phone_number` no secure storage local
- Solicitar permissao de push notifications apos verificacao bem-sucedida
- Timer de reenvio: 60 segundos
