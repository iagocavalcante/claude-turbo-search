# Tela: Splash / Welcome

**App:** Mobile Validador
**Prioridade:** MVP
**Plataformas:** iOS, Android

---

## Objetivo

Primeira tela ao abrir o app. Apresenta o proposito do app e direciona o usuario para registro.

---

## Estados

### 1. Primeiro Acesso (Onboarding)

```
┌─────────────────────────────┐
│                             │
│         [Logo App]          │
│                             │
│    Phone Validator          │
│                             │
│  ┌───────────────────────┐  │
│  │                       │  │
│  │   [Ilustracao/Icone]  │  │
│  │   Celular com escudo  │  │
│  │                       │  │
│  └───────────────────────┘  │
│                             │
│  Valide seu numero de       │
│  celular de forma rapida    │
│  e segura, sem SMS.         │
│                             │
│  ┌───────────────────────┐  │
│  │   Comecar Agora       │  │
│  └───────────────────────┘  │
│                             │
│  Ja tenho conta? Entrar     │
│                             │
└─────────────────────────────┘
```

### 2. Retorno (usuario ja registrado)

Splash screen por 1-2 segundos, depois redireciona automaticamente para Home.

```
┌─────────────────────────────┐
│                             │
│                             │
│         [Logo App]          │
│                             │
│    Phone Validator          │
│                             │
│       [Loading...]          │
│                             │
│                             │
└─────────────────────────────┘
```

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Primeiro acesso | Exibe onboarding com botao "Comecar Agora" |
| Tap "Comecar Agora" | Navega para Registro de Telefone |
| Retorno (ja registrado) | Splash 1.5s → Navega para Home |
| Retorno (token expirado) | Splash → Navega para Registro |

---

## Dados Necessarios

- Verificar se existe `device_token` e `phone_number` no storage local
- Verificar se o token FCM ainda e valido

---

## Notas Tecnicas

- Solicitar permissao de notificacoes push (iOS) nesta tela ou na proxima
- Registrar/atualizar token FCM ao abrir o app
- Background: cor solida do tema
