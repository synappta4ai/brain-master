# -*- coding: utf-8 -*-
r"""watch-announce - reconecta el gateway al worker de Colab sin copy/paste.

La celda 4 del notebook publica las variables de tunel en un topic FIJO de
ntfy.sh (ver NTFY_TOPIC); este watcher las consume y reinicia el gateway con
ellas, delegando en connect-gateway.ps1 (que ademas recompila el binario).

Validaciones anti-basura (ignora announces rotos o viejos):
  - PYTHON_WORKER_HOST tipo bore.pub:<puerto> con puerto en 1024-65535
  - BM_WORKER_ARTIFACT_BASE tipo https://<algo>.trycloudflare.com
    o http://bore.pub:<puerto> (variante Kaggle sin cloudflared)
  - announce con mas de STALE_SECS de antiguedad se ignora (sesion muerta)
  - nunca procesa dos veces el mismo announce (compara timestamp)

Seguridad: el topic es publico (como todo lo que viaja por ntfy.sh sin
registro); quien conozca el topic puede publicar announces falsos y el
gateway intentaria conectar alli. No viajan credenciales y connect-gateway
verifica health antes de dar por buena la conexion, pero para produccion
usa un relay privado o firma los mensajes.

Uso (desde la raiz del repo):
    py -3 deploy/colab/watch-announce.py            # en primer plano
    .\deploy\colab\watch-announce.ps1               # helper de Windows
    WATCH_DRY_RUN=1 py -3 ...                       # no toca el gateway
"""
import json
import os
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from urllib.request import urlopen

# Topic FIJO y commiteado: la celda 4 del notebook publica aqui.
# No es un secreto (no viajan credenciales; ver nota de seguridad arriba).
NTFY_TOPIC = "bm-brain-master-tunnels-v1"
NTFY_URL = f"https://ntfy.sh/{NTFY_TOPIC}/json?poll=1"
STALE_SECS = 600          # announces de sesiones muertas: ignorar
POLL_SECS = 30
DRY_RUN = os.environ.get("WATCH_DRY_RUN") == "1"

REPO = Path(__file__).resolve().parent.parent.parent
PS1 = REPO / "deploy" / "colab" / "connect-gateway.ps1"


def log(msg):
    print(f"[{datetime.now().strftime('%H:%M:%S')}] {msg}", flush=True)


def fetch_latest():
    """Trae los mensajes retenidos del topic y devuelve el ultimo."""
    try:
        with urlopen(NTFY_URL + "&since=12h", timeout=60) as r:
            lines = [json.loads(ln) for ln in r.read().decode().splitlines() if ln.strip()]
    except Exception as e:
        log(f"ntfy inaccesible ({e}); reintento en {POLL_SECS}s")
        return None
    msgs = [m for m in lines if m.get("event") == "message"]
    return msgs[-1] if msgs else None


def parse_and_validate(msg):
    """Devuelve (host, base) si el announce es valido y fresco; None si no."""
    try:
        data = json.loads(msg.get("message", ""))
    except json.JSONDecodeError:
        return None
    host = data.get("PYTHON_WORKER_HOST", "")
    base = data.get("BM_WORKER_ARTIFACT_BASE", "")
    port_str = host.split(":")[-1] if host.startswith("bore.pub:") else ""
    ok_host = port_str.isdigit() and 1024 <= int(port_str) <= 65535
    # Base de artefactos: cloudflared (Colab) o segundo bore (Kaggle, donde
    # la red mata QUIC/UDP de cloudflared).
    ok_base = (base.startswith("https://") and base.endswith(".trycloudflare.com")) or (
        base.startswith("http://bore.pub:") and base.split(":")[-1].isdigit()
    )
    if not (ok_host and ok_base):
        log(f"announce invalido ignorado: {host!r} | {base!r}")
        return None
    age = time.time() - msg.get("time", 0)
    if age > STALE_SECS:
        log(f"announce viejo ({int(age)}s) ignorado: sesion de Colab presumiblemente muerta")
        return None
    return host, base


def reconnect(host, base):
    log(f"reconectando gateway (background) -> {host} | {base}")
    if DRY_RUN:
        log(f"(dry-run) ejecutaria: connect-gateway.ps1 -Quick -WorkerHost {host} -ArtifactBase {base}")
        return True
    cmd = [
        "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass",
        "-File", str(PS1), "-Quick", "-WorkerHost", host, "-ArtifactBase", base,
    ]
    # Detached: el watcher NUNCA espera al ps1 (con el job de prueba podia
    # quedar bloqueado minutos y perder announces nuevos). El detalle queda
    # en backend/gateway-reconnect.log.
    logf = open(REPO / "backend" / "gateway-reconnect.log", "a")
    flags = 0x08000000  # CREATE_NO_WINDOW: sin consola pero proceso FUNCIONAL
    # (DETACHED_PROCESS mataba powershell silenciosamente antes de actuar)
    subprocess.Popen(cmd, stdout=logf, stderr=subprocess.STDOUT, creationflags=flags)
    log("ps1 lanzado detached (-Quick); detalle en backend/gateway-reconnect.log")
    return True


def main():
    log(f"watch-announce ON  topic={NTFY_TOPIC}" + ("  [DRY-RUN]" if DRY_RUN else ""))
    log("esperando la celda 4 de Colab (Ctrl+C para salir)...")
    last_ts = 0
    while True:
        msg = fetch_latest()
        if msg:
            ts = msg.get("time", 0)
            if ts > last_ts:            # announce nuevo (se marca visto aunque sea invalido)
                last_ts = ts
                parsed = parse_and_validate(msg)
                if parsed:
                    host, base = parsed
                    when = datetime.fromtimestamp(ts, tz=timezone.utc).strftime("%H:%M UTC")
                    log(f"announce nuevo ({when}): {host}")
                    reconnect(host, base)
                    log("vigilando de nuevo (proxima reconexion con la celda 4)")
        time.sleep(POLL_SECS)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\nhasta la proxima")
        sys.exit(0)
