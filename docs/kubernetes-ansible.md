# Kubernetes и Ansible

Документ описывает развёртывание SplitTheBill в Kubernetes и автоматизацию
деплоя через Ansible. Для локальной разработки по-прежнему используется
[`docker-compose.yml`](../docker-compose.yml).

## Назначение

Инфраструктура в [`infra/`](../infra) позволяет развернуть полный стек:

* PostgreSQL (`StatefulSet` с `volumeClaimTemplates`);
* backend (Go API, порт `8080`);
* web (Next.js standalone, порт `3000`);
* ingress nginx с маршрутами `/` и `/api/*`.

Ansible-плейбуки применяют манифесты, создают секреты из переменных и ждут
готовности rollout'ов. Для dev-кластера есть отдельный плейбук установки K3s.

## Структура `infra/`

```text
infra/
├── ansible/
│   ├── ansible.cfg
│   ├── inventory.example.ini
│   ├── requirements.yml
│   ├── group_vars/
│   │   └── all.example.yml
│   └── playbooks/
│       ├── deploy.yml
│       └── setup-k3s.yml
└── kubernetes/
    ├── base/
    │   ├── namespace.yaml
    │   ├── ingress.yaml
    │   ├── kustomization.yaml
    │   ├── postgres/
    │   │   ├── secret.example.yaml
    │   │   ├── service.yaml
    │   │   └── statefulset.yaml
    │   ├── backend/
    │   │   ├── configmap.yaml
    │   │   ├── secret.example.yaml
    │   │   ├── deployment.yaml
    │   │   └── service.yaml
    │   └── web/
    │       ├── deployment.yaml
    │       └── service.yaml
    └── overlays/
        ├── dev/
        │   └── kustomization.yaml
        └── prod/
            └── kustomization.yaml
```

## Требования

* `kubectl` (поддерживается `kubectl kustomize`, отдельный `kustomize` не
  обязателен);
* кластер Kubernetes с установленным nginx ingress controller;
* доступ к кластеру через `kubeconfig`;
* container registry для образов backend и web;
* `ansible-core` и коллекции из [`infra/ansible/requirements.yml`](../infra/ansible/requirements.yml)
  — только если используется Ansible.

## Сборка образов

```bash
docker build -t ghcr.io/OWNER/splitthebill-backend:latest ./backend
docker build \
  --build-arg NEXT_PUBLIC_API_URL=/api \
  -t ghcr.io/OWNER/splitthebill-web:latest ./web
docker push ghcr.io/OWNER/splitthebill-backend:latest
docker push ghcr.io/OWNER/splitthebill-web:latest
```

Замените `OWNER` на свой registry/namespace и обновите `images` в
[`infra/kubernetes/overlays/dev/kustomization.yaml`](../infra/kubernetes/overlays/dev/kustomization.yaml)
или [`infra/kubernetes/overlays/prod/kustomization.yaml`](../infra/kubernetes/overlays/prod/kustomization.yaml).
Образы можно переопределить и без правки файлов:

```bash
kubectl apply -k infra/kubernetes/overlays/prod
kubectl -n splitthebill set image deployment/backend \
  backend=ghcr.io/OWNER/splitthebill-backend:v1.2.3
kubectl -n splitthebill set image deployment/web \
  web=ghcr.io/OWNER/splitthebill-web:v1.2.3
```

### `NEXT_PUBLIC_API_URL`

`NEXT_PUBLIC_API_URL` встраивается в клиентский бандл на этапе `next build`,
поэтому менять его в Deployment бесполезно — нужно пересобирать образ web.

**Рекомендуемый подход** — относительный URL:

```text
NEXT_PUBLIC_API_URL=/api
```

Frontend обращается к `/api/rooms`, а ingress отправляет `/api/*` в backend
и снимает префикс (`rewrite-target: /$2`). Внешний домен не вшивается в образ,
один и тот же образ работает на dev и prod.

**Альтернативный подход** — отдельный домен API:

```bash
docker build \
  --build-arg NEXT_PUBLIC_API_URL=https://api.example.com \
  -t ghcr.io/OWNER/splitthebill-web:latest ./web
```

В этом случае ingress для `/api` не обязателен, но нужен отдельный ingress
для `api.example.com` и корректный `ALLOWED_ORIGINS`.

## Секреты

В репозитории хранятся только файлы-примеры:

* [`infra/kubernetes/base/postgres/secret.example.yaml`](../infra/kubernetes/base/postgres/secret.example.yaml);
* [`infra/kubernetes/base/backend/secret.example.yaml`](../infra/kubernetes/base/backend/secret.example.yaml).

Создание секретов вручную:

```bash
cp infra/kubernetes/base/postgres/secret.example.yaml \
  infra/kubernetes/base/postgres/secret.yaml
cp infra/kubernetes/base/backend/secret.example.yaml \
  infra/kubernetes/base/backend/secret.yaml
# отредактируйте CHANGE_ME в обоих файлах (пароль должен совпадать)
kubectl apply -f infra/kubernetes/base/postgres/secret.yaml
kubectl apply -f infra/kubernetes/base/backend/secret.yaml
```

Правила:

* реальные `secret.yaml` не коммитятся — они в [`.gitignore`](../.gitignore);
* в git допустимы только `*.example.yaml`;
* для production используйте Ansible Vault, внешний secret manager или
  переменные окружения CI.

Ansible-плейбук создаёт секреты из `postgres_password` и `database_url`
(`no_log: true`) и не пишет их на диск.

## Применение манифестов

Сначала примените секреты, затем манифесты:

```bash
kubectl apply -k infra/kubernetes/base
# или overlay:
kubectl apply -k infra/kubernetes/overlays/prod
```

`kustomize build` не требует наличия реальных секретов: они не входят в
`kustomization.yaml`, а Deployment/StatefulSet ссылаются на них по имени.
Если секретов нет, поды не запустятся (`CreateContainerConfigError`), поэтому
создайте их до `apply`.

Проверить результат сборки без кластера:

```bash
kubectl kustomize infra/kubernetes/base
kubectl kustomize infra/kubernetes/overlays/dev
kubectl kustomize infra/kubernetes/overlays/prod
```

### Отличия overlays

* `dev` — теги `dev`, домен `dev.app.example.com`, отдельный `ALLOWED_ORIGINS`;
* `prod` — теги `latest`, 2 реплики backend и web, увеличенные resources,
  домен `app.example.com`.

Overlays используют namespace `splitthebill`. Если нужно другое имя
namespace, измените его в `kustomization.yaml` overlay и в
`group_vars/all.yml`.

## Ansible

```bash
cd infra/ansible
ansible-galaxy collection install -r requirements.yml
cp inventory.example.ini inventory.ini
cp group_vars/all.example.yml group_vars/all.yml
# отредактируйте inventory.ini и group_vars/all.yml
ansible-playbook -i inventory.ini playbooks/deploy.yml
```

`deploy.yml`:

1. проверяет наличие `kubectl`;
2. создаёт namespace;
3. создаёт Secrets `splitthebill-postgres` и `splitthebill-backend`
   из переменных;
4. применяет overlay (`app_overlay`, по умолчанию `prod`);
5. переопределяет образы из `backend_image` и `web_image`;
6. ждёт `rollout status` для PostgreSQL, backend и web.

Пароль нельзя оставлять `CHANGE_ME` — плейбук остановится. Передавайте его так:

```bash
ansible-playbook -i inventory.ini playbooks/deploy.yml \
  --extra-vars "postgres_password=$(pass show splitthebill/postgres)"
```

или храните в Ansible Vault:

```bash
ansible-vault encrypt_string --name postgres_password
```

Плейбук рассчитан на запуск с машины, где есть checkout репозитория,
`kubectl` и `kubeconfig`. В примере inventory у control-хоста указан
`ansible_connection=local`.

### Dev-кластер K3s

```bash
ansible-playbook -i inventory.ini playbooks/setup-k3s.yml
```

Плейбук **только для разработки**: ставит однонодовый K3s, копирует
`/etc/rancher/k3s/k3s.yaml` в `~/.kube/config` и проверяет `kubectl get nodes`.
Для production используйте управляемый кластер.

## Проверка развёртывания

```bash
kubectl get pods -n splitthebill
kubectl logs -n splitthebill deployment/backend
kubectl rollout status -n splitthebill deployment/backend
curl https://app.example.com/health
curl https://app.example.com/api/health
```

Ожидаемые ответы:

* `GET /health` web — `200 ok` (route `web/app/health/route.ts`);
* `GET /api/health` — `200 ok` от backend.

## Ограничения и замечания

* PostgreSQL в Kubernetes — один StatefulSet с `ReadWriteOnce` PVC. Для
  production рекомендуется managed PostgreSQL с резервными копиями и
  репликацией.
* В манифестах нет TLS, cert-manager, NetworkPolicy, PodDisruptionBudget,
  HPA и мониторинга.
* Нет автоматической сборки и публикации образов из Ansible/CI — образы
  должны быть собраны и запушены отдельно.
* Ingress требует установленного nginx ingress controller и DNS-записи для
  домена.
* Namespace `splitthebill` задан в kustomization; overlays не создают
  отдельные namespace для dev и prod.
* Ansible не управляет кластером (не устанавливает ingress controller) —
  предполагается, что кластер уже подготовлен.
* `docker-compose` остаётся основным способом локального запуска; Kubernetes
  и Ansible не влияют на него и на `render.yaml`.
