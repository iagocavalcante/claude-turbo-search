# Tela: Registro de Telefone

**App:** Mobile Validador
**Prioridade:** MVP
**Plataformas:** iOS, Android

---

## Objetivo

Coletar o numero de celular do usuario para registrar o dispositivo como validador.

---

## Layout

```
┌─────────────────────────────┐
│ ←                           │
│                             │
│  Registre seu numero        │
│                             │
│  Informe o numero do        │
│  celular que sera usado     │
│  para validacoes.           │
│                             │
│  Pais                       │
│  ┌───────────────────────┐  │
│  │ 🇧🇷 Brasil (+55)    ▼ │  │
│  └───────────────────────┘  │
│                             │
│  Numero de celular          │
│  ┌───────────────────────┐  │
│  │ (11) 99999-9999       │  │
│  └───────────────────────┘  │
│                             │
│  Voce recebera um codigo    │
│  SMS para confirmar que     │
│  este numero e seu.         │
│                             │
│  ┌───────────────────────┐  │
│  │   Continuar            │  │
│  └───────────────────────┘  │
│                             │
└─────────────────────────────┘
```

---

## Campos

| Campo | Tipo | Validacao | Obrigatorio |
|-------|------|-----------|-------------|
| Pais | Dropdown/Picker | Lista de paises com DDI | Sim |
| Numero | Phone input | Formato valido E.164 | Sim |

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Selecionar pais | Atualiza mascara do numero e DDI |
| Digitar numero | Aplica mascara do pais selecionado |
| Tap "Continuar" (valido) | Envia SMS com OTP → Navega para Verificacao |
| Tap "Continuar" (invalido) | Exibe erro inline "Numero invalido" |
| Tap "←" (voltar) | Retorna para Splash |

---

## Validacoes

1. Numero deve ter quantidade correta de digitos para o pais
2. Formato E.164 valido (ex: `+5511999999999`)
3. Nao permitir numeros fixos (apenas celular)
4. Rate limit: maximo 3 tentativas de registro por hora

---

## API Call

```
POST /internal/devices/verify
{
  "phone_number": "+5511999999999"
}

Response 200:
{
  "verification_id": "uuid",
  "expires_in": 300
}
```

---

## Tratamento de Erros

| Erro | Mensagem |
|------|----------|
| Numero invalido | "Numero de celular invalido. Verifique e tente novamente." |
| Rate limit | "Muitas tentativas. Aguarde alguns minutos." |
| Rede indisponivel | "Sem conexao. Verifique sua internet." |
| Erro servidor | "Erro interno. Tente novamente em instantes." |

---

## Notas Tecnicas

- Usar biblioteca de validacao de telefone (ex_phone_number no backend)
- Mascara de input por pais
- Keyboard: numerico
- Auto-detectar pais pelo SIM card ou locale do dispositivo
