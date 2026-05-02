# Tela: Configuracoes

**App:** Mobile Validador
**Prioridade:** MVP
**Plataformas:** iOS, Android

---

## Objetivo

Permitir ao usuario gerenciar configuracoes do app, notificacoes e conta.

---

## Layout

```
┌─────────────────────────────┐
│  Configuracoes              │
│─────────────────────────────│
│                             │
│  CONTA                      │
│  ┌───────────────────────┐  │
│  │ Numero Registrado     │  │
│  │ +55 11 99999-9999   → │  │
│  ├───────────────────────┤  │
│  │ Status do Dispositivo │  │
│  │ Ativo               → │  │
│  └───────────────────────┘  │
│                             │
│  NOTIFICACOES               │
│  ┌───────────────────────┐  │
│  │ Push Notifications    │  │
│  │ Ativadas         [●] │  │
│  ├───────────────────────┤  │
│  │ Som de Notificacao    │  │
│  │ Ativado          [●] │  │
│  ├───────────────────────┤  │
│  │ Vibracao              │  │
│  │ Ativada          [●] │  │
│  └───────────────────────┘  │
│                             │
│  SEGURANCA                  │
│  ┌───────────────────────┐  │
│  │ Biometria para        │  │
│  │ Confirmar         [○] │  │
│  └───────────────────────┘  │
│                             │
│  SOBRE                      │
│  ┌───────────────────────┐  │
│  │ Versao do App         │  │
│  │ 1.0.0                 │  │
│  ├───────────────────────┤  │
│  │ Termos de Uso       → │  │
│  ├───────────────────────┤  │
│  │ Politica de           │  │
│  │ Privacidade         → │  │
│  └───────────────────────┘  │
│                             │
│  ┌───────────────────────┐  │
│  │  Trocar Numero        │  │
│  └───────────────────────┘  │
│                             │
│  ┌───────────────────────┐  │
│  │  Desregistrar         │  │
│  │  Dispositivo          │  │
│  └───────────────────────┘  │
│                             │
│─────────────────────────────│
│  [Home]    [Historico]  [⚙️] │
└─────────────────────────────┘
```

---

## Secoes

### CONTA

| Item | Tipo | Acao |
|------|------|------|
| Numero Registrado | Navegacao | Exibe info do numero, opcao de trocar |
| Status do Dispositivo | Navegacao | Exibe detalhes (token, plataforma, ultimo uso) |

### NOTIFICACOES

| Item | Tipo | Padrao | Descricao |
|------|------|--------|-----------|
| Push Notifications | Toggle | ON | Ativar/desativar push (redireciona para settings do OS) |
| Som de Notificacao | Toggle | ON | Som ao receber validacao |
| Vibracao | Toggle | ON | Vibrar ao receber validacao |

### SEGURANCA

| Item | Tipo | Padrao | Descricao |
|------|------|--------|-----------|
| Biometria para Confirmar | Toggle | OFF | Exigir Face ID / Touch ID / Fingerprint para confirmar validacao |

### SOBRE

| Item | Tipo | Descricao |
|------|------|-----------|
| Versao do App | Info | Exibe versao atual |
| Termos de Uso | Link | Abre WebView com termos |
| Politica de Privacidade | Link | Abre WebView com politica |

### ACOES

| Item | Tipo | Descricao |
|------|------|-----------|
| Trocar Numero | Botao (outline) | Inicia fluxo de troca de numero |
| Desregistrar Dispositivo | Botao (destructive) | Remove registro, volta para Splash |

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Toggle Push ON | Verifica permissao, redireciona para settings se necessario |
| Toggle Push OFF | Alerta "Voce nao recebera validacoes" → Desabilita |
| Toggle Biometria ON | Solicita autenticacao biometrica para ativar |
| Tap "Trocar Numero" | Confirmacao → Fluxo de Registro de Telefone |
| Tap "Desregistrar" | Confirmacao dupla → Remove dados → Splash |

---

## Dialogo: Desregistrar Dispositivo

```
┌───────────────────────────┐
│  Desregistrar?            │
│                           │
│  Seu numero sera removido │
│  e voce nao recebera mais │
│  solicitacoes de validacao.│
│                           │
│  [Cancelar]  [Desregistrar]│
└───────────────────────────┘
```

---

## API Calls

### Desregistrar
```
DELETE /internal/devices/{device_id}
Headers:
  X-Device-Token: device_token_here

Response 204: No Content
```

### Atualizar Configuracoes
```
PATCH /internal/devices/{device_id}/settings
{
  "notifications_enabled": true,
  "sound_enabled": true,
  "vibration_enabled": true,
  "biometric_required": false
}

Response 200:
{
  "id": "uuid",
  "settings": { ... }
}
```

---

## Notas Tecnicas

- Configuracoes de som e vibracao sao locais (nao precisam de API)
- Biometria usa LocalAuthentication (iOS) / BiometricPrompt (Android)
- "Desregistrar" deve limpar todos os dados locais (secure storage, cache, etc)
- Push toggle reflete estado real do OS, nao apenas do app
