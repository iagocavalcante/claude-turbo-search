# Tela: Validacao Recebida (Push)

**App:** Mobile Validador
**Prioridade:** MVP
**Plataformas:** iOS, Android

---

## Objetivo

Tela principal de interacao. Exibida quando o usuario recebe um push notification solicitando validacao do numero. O usuario deve confirmar ou recusar.

---

## Layout

```
┌─────────────────────────────┐
│                             │
│                             │
│     ┌─────────────────┐     │
│     │   [Icone/Logo]  │     │
│     │   Empresa XYZ   │     │
│     └─────────────────┘     │
│                             │
│  Solicitacao de Validacao   │
│                             │
│  A empresa Empresa XYZ      │
│  quer confirmar que este    │
│  numero e seu.              │
│                             │
│  ┌───────────────────────┐  │
│  │                       │  │
│  │  +55 11 99999-9999    │  │
│  │                       │  │
│  └───────────────────────┘  │
│                             │
│  Expira em: 4:32            │
│                             │
│                             │
│  ┌───────────────────────┐  │
│  │   ✓  Confirmar        │  │
│  └───────────────────────┘  │
│                             │
│  ┌───────────────────────┐  │
│  │   ✗  Recusar          │  │
│  └───────────────────────┘  │
│                             │
│  Nao reconhece? Isto pode  │
│  ser uma tentativa nao      │
│  autorizada.                │
│                             │
└─────────────────────────────┘
```

---

## Estados

### 1. Aguardando Resposta (padrao)
- Timer de expiracao visivel (countdown)
- Botoes "Confirmar" e "Recusar" ativos

### 2. Confirmando
```
┌───────────────────────┐
│   [Loading...]        │
│   Confirmando...      │
└───────────────────────┘
```
- Botoes desabilitados
- Spinner no botao "Confirmar"

### 3. Confirmado com Sucesso
```
┌─────────────────────────────┐
│                             │
│     ┌─────────────────┐     │
│     │    ✓ Grande     │     │
│     │    (animado)    │     │
│     └─────────────────┘     │
│                             │
│  Numero Validado!           │
│                             │
│  Sua identidade foi         │
│  confirmada com sucesso.    │
│                             │
│  ┌───────────────────────┐  │
│  │   Voltar ao Inicio    │  │
│  └───────────────────────┘  │
│                             │
└─────────────────────────────┘
```

### 4. Recusado
```
┌─────────────────────────────┐
│                             │
│     ┌─────────────────┐     │
│     │    ✗ Grande     │     │
│     │    (animado)    │     │
│     └─────────────────┘     │
│                             │
│  Validacao Recusada         │
│                             │
│  A solicitacao foi          │
│  recusada.                  │
│                             │
│  ┌───────────────────────┐  │
│  │   Voltar ao Inicio    │  │
│  └───────────────────────┘  │
│                             │
└─────────────────────────────┘
```

### 5. Expirado
```
┌─────────────────────────────┐
│                             │
│     ┌─────────────────┐     │
│     │    ⏰ Grande    │     │
│     └─────────────────┘     │
│                             │
│  Solicitacao Expirada       │
│                             │
│  O tempo para confirmar     │
│  esta validacao expirou.    │
│                             │
│  ┌───────────────────────┐  │
│  │   Voltar ao Inicio    │  │
│  └───────────────────────┘  │
│                             │
└─────────────────────────────┘
```

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Tap "Confirmar" | Envia confirmacao → Estado "Confirmado" |
| Tap "Recusar" | Envia recusa → Estado "Recusado" |
| Timer chega a 0:00 | Estado "Expirado" |
| Tap "Voltar ao Inicio" | Navega para Home |
| App em background + push | Notificacao do OS com acoes rapidas |

---

## Push Notification (entrada)

### Foreground
- Exibe esta tela diretamente sobre qualquer tela atual

### Background / Locked
```
┌─────────────────────────────┐
│ Phone Validator             │
│ Solicitacao de validacao    │
│ Empresa XYZ quer confirmar  │
│ seu numero.                 │
│                             │
│ [Confirmar]     [Recusar]   │
└─────────────────────────────┘
```
- Acoes rapidas na notificacao (iOS: actionable notification, Android: notification actions)
- Tap na notificacao abre a tela completa

---

## API Call

### Confirmar Validacao
```
POST /internal/validations/{id}/confirm
{
  "guid": "uuid-recebido-no-push"
}

Response 200:
{
  "id": "uuid",
  "status": "validated",
  "validated_at": "2026-02-06T15:01:30Z"
}
```

### Recusar Validacao
```
POST /internal/validations/{id}/reject
{
  "guid": "uuid-recebido-no-push",
  "reason": "not_recognized"
}

Response 200:
{
  "id": "uuid",
  "status": "rejected"
}
```

---

## Dados do Push Payload

```json
{
  "type": "validation_request",
  "validation_id": "uuid",
  "guid": "uuid",
  "phone_number": "+5511999999999",
  "client_name": "Empresa XYZ",
  "expires_at": "2026-02-06T15:05:00Z"
}
```

---

## Tratamento de Erros

| Erro | Mensagem |
|------|----------|
| Validacao nao encontrada | "Esta solicitacao nao existe mais." |
| Validacao ja confirmada | "Esta solicitacao ja foi confirmada." |
| GUID incorreto | "Erro de seguranca. Tente novamente." |
| Rede indisponivel | "Sem conexao. Verifique sua internet e tente novamente." |
| Expirada ao confirmar | "O tempo expirou. Solicite nova validacao." |

---

## Notas Tecnicas

- Timer de countdown sincronizado com `expires_at` do servidor
- Armazenar GUID localmente para confirmar (recebido no push payload)
- Animacoes de sucesso/erro para feedback visual claro
- Haptic feedback ao confirmar/recusar
- Som sutil ao receber solicitacao (configuravel)
- Tela deve funcionar vindo de push notification (cold start e warm start)
- Auto-dismiss apos 5 segundos no estado de sucesso/erro
