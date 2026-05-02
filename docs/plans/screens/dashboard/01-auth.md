# Tela: Login / Cadastro

**App:** Dashboard Web (Cliente SaaS)
**Prioridade:** MVP
**Tecnologia:** Phoenix LiveView

---

## Objetivo

Autenticacao de clientes (empresas) que usam a API de validacao.

---

## Layout: Login

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│              ┌─────────────────────────┐                    │
│              │                         │                    │
│              │    [Logo]               │                    │
│              │    Phone Validator      │                    │
│              │                         │                    │
│              │  Email                  │                    │
│              │  ┌───────────────────┐  │                    │
│              │  │                   │  │                    │
│              │  └───────────────────┘  │                    │
│              │                         │                    │
│              │  Senha                  │                    │
│              │  ┌───────────────────┐  │                    │
│              │  │                   │  │                    │
│              │  └───────────────────┘  │                    │
│              │                         │                    │
│              │  [ ] Lembrar de mim     │                    │
│              │                         │                    │
│              │  ┌───────────────────┐  │                    │
│              │  │     Entrar        │  │                    │
│              │  └───────────────────┘  │                    │
│              │                         │                    │
│              │  Esqueceu a senha?      │                    │
│              │                         │                    │
│              │  ─────── ou ────────    │                    │
│              │                         │                    │
│              │  Nao tem conta?         │                    │
│              │  Criar conta gratis     │                    │
│              │                         │                    │
│              └─────────────────────────┘                    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Layout: Cadastro

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│              ┌─────────────────────────┐                    │
│              │                         │                    │
│              │    [Logo]               │                    │
│              │    Criar Conta          │                    │
│              │                         │                    │
│              │  Nome da Empresa        │                    │
│              │  ┌───────────────────┐  │                    │
│              │  │                   │  │                    │
│              │  └───────────────────┘  │                    │
│              │                         │                    │
│              │  Seu Nome               │                    │
│              │  ┌───────────────────┐  │                    │
│              │  │                   │  │                    │
│              │  └───────────────────┘  │                    │
│              │                         │                    │
│              │  Email                  │                    │
│              │  ┌───────────────────┐  │                    │
│              │  │                   │  │                    │
│              │  └───────────────────┘  │                    │
│              │                         │                    │
│              │  Senha                  │                    │
│              │  ┌───────────────────┐  │                    │
│              │  │                   │  │                    │
│              │  └───────────────────┘  │                    │
│              │                         │                    │
│              │  Confirmar Senha        │                    │
│              │  ┌───────────────────┐  │                    │
│              │  │                   │  │                    │
│              │  └───────────────────┘  │                    │
│              │                         │                    │
│              │  [ ] Concordo com os    │                    │
│              │  Termos de Servico      │                    │
│              │                         │                    │
│              │  ┌───────────────────┐  │                    │
│              │  │   Criar Conta     │  │                    │
│              │  └───────────────────┘  │                    │
│              │                         │                    │
│              │  Ja tem conta? Entrar   │                    │
│              │                         │                    │
│              └─────────────────────────┘                    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Campos

### Login

| Campo | Tipo | Validacao | Obrigatorio |
|-------|------|-----------|-------------|
| Email | email | Formato valido | Sim |
| Senha | password | Min 8 chars | Sim |
| Lembrar | checkbox | - | Nao |

### Cadastro

| Campo | Tipo | Validacao | Obrigatorio |
|-------|------|-----------|-------------|
| Nome da Empresa | text | Min 2 chars | Sim |
| Seu Nome | text | Min 2 chars | Sim |
| Email | email | Formato valido, unico | Sim |
| Senha | password | Min 8 chars, 1 numero, 1 maiuscula | Sim |
| Confirmar Senha | password | Igual a senha | Sim |
| Termos | checkbox | Deve aceitar | Sim |

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Login valido | Redireciona para Dashboard |
| Login invalido | Exibe "Email ou senha incorretos" |
| Cadastro valido | Envia email de confirmacao → Tela "Verifique seu email" |
| Tap "Esqueceu a senha?" | Tela de recuperacao de senha |
| Tap "Criar conta" | Alterna para formulario de cadastro |
| Tap "Entrar" | Alterna para formulario de login |

---

## Tela: Verifique seu Email

```
┌─────────────────────────┐
│                         │
│    [Icone Email]        │
│                         │
│  Verifique seu email    │
│                         │
│  Enviamos um link de    │
│  confirmacao para       │
│  user@empresa.com       │
│                         │
│  Nao recebeu?           │
│  Reenviar email         │
│                         │
└─────────────────────────┘
```

---

## Notas Tecnicas

- Usar phx_gen_auth para autenticacao
- Sessao via cookie (Phoenix sessions)
- Rate limit: 5 tentativas de login por IP por minuto
- Senha hasheada com bcrypt
- Email de confirmacao via sistema de emails (SendGrid, SES, etc)
- CSRF protection via Phoenix default
