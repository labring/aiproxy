/* Standalone voice-input demo. No framework, persisted credentials or browser speech service. */
(() => {
  "use strict";
  const $ = (id) => document.getElementById(id);
  const editor = $("editor"),
    mic = $("mic"),
    cancelButton = $("cancel");
  let current = null,
    state = "idle",
    generation = 0,
    hold = false;
  const statusLabels = {
    idle: "点击麦克风，开始语音输入",
    starting: "正在连接，准备聆听…",
    listening: "正在聆听",
    finishing: "正在确认最后一句…",
  };
  const localDemoOrigin = "http://127.0.0.1:3810";
  const hasWebOrigin = ["http:", "https:"].includes(location.protocol);
  $("endpoint").value = hasWebOrigin ? location.origin : localDemoOrigin;
  let checkingConnection = false;
  let gatewayDemoOrigin;
  const expectedOpenerOrigin = new URLSearchParams(location.search).get(
    "opener_origin",
  );

  // A separately opened HTML can hand off to the gateway's own demo without putting keys in URLs/storage.
  function openHostedDemo(origin) {
    const target = new URL("/demo/realtime-voice.html", origin);
    target.searchParams.set(
      "opener_origin",
      location.protocol === "file:" ? "null" : location.origin,
    );
    const config = {
      type: "aiproxy.voice.config",
      key: $("apiKey").value.trim(),
      model: $("model").value.trim(),
      draft: editor.textContent,
    };
    const popup = window.open(target.href, "_blank");
    if (!popup) {
      error("浏览器阻止了新窗口，请允许弹出窗口后重试。");
      return;
    }
    let attempts = 0;
    // The target may still be loading. Only that exact origin can receive the in-memory configuration.
    const timer = setInterval(() => {
      if (popup.closed || ++attempts > 75) {
        clearInterval(timer);
        return;
      }
      popup.postMessage(config, target.origin);
    }, 200);
  }
  window.addEventListener("message", (event) => {
    if (
      !window.opener ||
      event.source !== window.opener ||
      !expectedOpenerOrigin ||
      event.origin !== expectedOpenerOrigin ||
      event.data?.type !== "aiproxy.voice.config"
    )
      return;
    const { key, model, draft } = event.data;
    if (typeof key !== "string" || typeof model !== "string") return;
    $("apiKey").value = key;
    $("model").value = model;
    $("endpoint").value = location.origin;
    if (typeof draft === "string") editor.textContent = draft;
    $("config").open = false;
    window.opener = null;
    history.replaceState(null, "", location.pathname);
    updateState("idle");
  });
  $("openGatewayDemo").addEventListener("click", () => {
    if (gatewayDemoOrigin) openHostedDemo(gatewayDemoOrigin);
  });
  $("useLocalDemo").addEventListener("click", () => {
    $("endpoint").value = localDemoOrigin;
    if (location.origin === localDemoOrigin) {
      error("");
      $("config").open = false;
    } else openHostedDemo(localDemoOrigin);
  });
  const processorSource = `class VoicePCM extends AudioWorkletProcessor {
    constructor(){super();this.phase=0;this.sum=0;this.count=0;this.samples=[];this.port.onmessage=e=>{if(e.data==='flush'){this.flush();this.port.postMessage({flushed:true});}};}
    flush(){if(!this.samples.length)return;const pcm=new Int16Array(this.samples);this.samples=[];this.port.postMessage({pcm:pcm.buffer},[pcm.buffer]);}
    process(inputs){const data=inputs[0]?.[0];if(data)for(const value of data){this.sum+=value;this.count++;this.phase+=16000;if(this.phase>=sampleRate){const v=Math.max(-1,Math.min(1,this.sum/this.count));this.samples.push(Math.round(v*(v<0?32768:32767)));this.phase-=sampleRate;this.sum=0;this.count=0;if(this.samples.length>=1600)this.flush();}}return true;}
  } registerProcessor('voice-pcm',VoicePCM);`;
  function updateState(next) {
    state = next;
    mic.classList.toggle("active", next !== "idle");
    mic.setAttribute(
      "aria-label",
      next === "idle" ? "开始语音输入" : "结束语音输入",
    );
    mic.setAttribute("aria-pressed", String(next !== "idle"));
    $("statusText").textContent = statusLabels[next];
    $("status").classList.toggle("live", next === "listening");
    cancelButton.hidden = next === "idle";
    $("elapsed").hidden = next === "idle";
    $("send").disabled = next !== "idle" || !editor.textContent.trim();
  }
  function error(message) {
    $("error").textContent = message;
    $("connectionActions").hidden = true;
  }
  function audioCleanup(session) {
    session.stream?.getTracks().forEach((t) => t.stop());
    session.source?.disconnect();
    session.node?.disconnect();
    if (session.context && session.context.state !== "closed")
      void session.context.close();
    clearInterval(session.timer);
    clearTimeout(session.finishTimer);
  }
  function renderItem(session, event) {
    if (!session.anchor.isConnected) {
      void stop(true);
      return;
    }
    const id = event.item_id;
    if (!id) return;
    let item = session.items.get(id);
    if (!item) {
      item = { element: document.createElement("span"), final: false };
      item.element.dataset.voiceItem = id;
      session.anchor.append(item.element);
      session.items.set(id, item);
    }
    if (item.final) return;
    if (event.type.endsWith(".completed")) {
      item.element.textContent = event.transcript || "";
      item.final = true;
    } else {
      const text = document.createTextNode(event.text || "");
      const pending = document.createElement("span");
      pending.className = "pending";
      pending.dataset.voicePending = "true";
      pending.textContent = event.stash || "";
      item.element.replaceChildren(text, pending);
    }
    session.events++;
    $("stats").textContent = `实时更新 ${session.events} 次`;
    editor.scrollTop = editor.scrollHeight;
  }
  function settle(session, cancelled) {
    if (current !== session) return;
    session.audioClosed = true;
    audioCleanup(session);
    session.socket?.close();
    if (cancelled) {
      session.anchor.replaceWith(session.original);
    } else {
      session.anchor
        .querySelectorAll(".pending")
        .forEach((x) => x.replaceWith(...x.childNodes));
      session.anchor.replaceWith(...session.anchor.childNodes);
    }
    current = null;
    generation++;
    updateState("idle");
    if (!cancelled) {
      editor.focus();
      const range = document.createRange();
      range.selectNodeContents(editor);
      range.collapse(false);
      const selection = getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
    }
  }
  function insertAnchor() {
    const selection = getSelection();
    let range;
    if (
      selection?.rangeCount &&
      editor.contains(selection.anchorNode) &&
      editor.contains(selection.focusNode)
    ) {
      range = selection.getRangeAt(0).cloneRange();
    } else {
      range = document.createRange();
      range.selectNodeContents(editor);
      range.collapse(false);
    }
    const original = range.extractContents();
    const anchor = document.createElement("span");
    anchor.dataset.voiceSession = "true";
    anchor.contentEditable = "false";
    range.insertNode(anchor);
    range.setStartAfter(anchor);
    range.collapse(true);
    selection?.removeAllRanges();
    selection?.addRange(range);
    return { anchor, original };
  }
  async function start(mode) {
    if (state !== "idle" || checkingConnection) return;
    error("");
    const key = $("apiKey").value.trim(),
      model = $("model").value.trim();
    let endpoint;
    try {
      endpoint = new URL($("endpoint").value);
      if (endpoint.protocol === "ws:") endpoint.protocol = "http:";
      if (endpoint.protocol === "wss:") endpoint.protocol = "https:";
      if (
        !["http:", "https:"].includes(endpoint.protocol) ||
        endpoint.username ||
        endpoint.password
      )
        throw Error("请填写不含 Key 的 HTTP 或 HTTPS 服务地址。");
      if (!key) throw Error("请先在连接设置中填写 AI Proxy API Key。");
      const loopback = (host) =>
        ["localhost", "127.0.0.1", "[::1]"].includes(host);
      if (
        hasWebOrigin &&
        loopback(endpoint.hostname) &&
        loopback(location.hostname) &&
        endpoint.port === location.port &&
        endpoint.protocol === location.protocol
      ) {
        endpoint = new URL(location.origin);
        $("endpoint").value = endpoint.origin;
      }
      if (endpoint.origin !== location.origin) {
        checkingConnection = true;
        let available = false;
        let unavailableStatus;
        try {
          const response = await fetch(
            new URL("/v1/audio/realtime", endpoint),
            {
              credentials: "omit",
              referrerPolicy: "no-referrer",
              signal: AbortSignal.timeout(6000),
            },
          );
          unavailableStatus = response.status;
          const body = await response.json();
          available =
            response.status === 400 &&
            body.error === "WebSocket upgrade required";
        } catch {
          /* A direct same-origin page can also resolve a target's CORS restriction. */
        } finally {
          checkingConnection = false;
        }
        error(
          available
            ? "此服务支持实时语音。点击“打开该服务的实时演示”，连接信息会自动带入。"
            : unavailableStatus === 404
              ? "目标 AI Proxy 尚未部署实时接口（404）。可先使用本机新版演示，远程 dev 需要升级实时接口版本。"
              : "暂时无法访问该服务的实时接口。可先使用本机新版演示。",
        );
        gatewayDemoOrigin = endpoint.origin;
        $("openGatewayDemo").hidden = !available;
        $("connectionActions").hidden = false;
        $("config").open = true;
        return;
      }
      if (!navigator.mediaDevices?.getUserMedia || !window.AudioWorkletNode)
        throw Error(
          "当前浏览器不支持录音，请使用 HTTPS 或 localhost 下的新版 Chrome。",
        );
    } catch (e) {
      error(e.message);
      $("config").open = true;
      return;
    }
    const session = {
      id: ++generation,
      mode,
      items: new Map(),
      events: 0,
      startedAt: Date.now(),
      socket: null,
      ...insertAnchor(),
    };
    current = session;
    updateState("starting");
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
        },
        video: false,
      });
      session.stream = stream;
      if (current !== session) {
        stream.getTracks().forEach((t) => t.stop());
        return;
      }
      if (session.stopRequested) {
        settle(session, false);
        return;
      }
      const url = new URL("/v1/audio/realtime", endpoint);
      url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
      url.searchParams.set("model", model);
      const socket = new WebSocket(url, [
        "aiproxy-realtime",
        "aiproxy-api-key." + key,
      ]);
      session.socket = socket;
      await new Promise((resolve, reject) => {
        session.rejectStart = reject;
        const timeout = setTimeout(
          () => reject(Error("实时连接超时，请检查渠道和模型配置。")),
          15000,
        );
        session.startTimeout = timeout;
        socket.onopen = () =>
          socket.send(
            JSON.stringify({
              event_id: crypto.randomUUID(),
              type: "session.update",
              session: {
                input_audio_format: "pcm",
                sample_rate: 16000,
                input_audio_transcription: { language: "zh" },
                turn_detection: {
                  type: "server_vad",
                  threshold: 0,
                  silence_duration_ms: 600,
                },
              },
            }),
          );
        socket.onerror = () =>
          reject(Error("连接失败，请检查 AI Proxy Key、模型权限及渠道。"));
        socket.onmessage = ({ data }) => {
          if (current !== session) return;
          let event;
          try {
            event = JSON.parse(data);
          } catch {
            return;
          }
          if (event.type === "session.updated") {
            clearTimeout(timeout);
            resolve();
          }
          if (
            event.type === "conversation.item.input_audio_transcription.text" ||
            event.type ===
              "conversation.item.input_audio_transcription.completed"
          )
            renderItem(session, event);
          if (
            event.type === "error" ||
            event.type.endsWith("transcription.failed")
          ) {
            const message = event.error?.message || "语音识别失败";
            error(message);
            reject(Error(message));
            settle(session, false);
          }
          if (event.type === "session.finished") {
            clearTimeout(timeout);
            if (event.usage)
              $("stats").textContent =
                `本次音频 ${event.usage.seconds} 秒 · ${session.events} 次实时更新`;
            settle(session, false);
          }
        };
        socket.onclose = () => {
          clearTimeout(timeout);
          if (current === session) {
            if (state !== "finishing")
              error("实时连接已断开，已保留收到的文字。");
            reject(Error("实时连接已关闭"));
            settle(session, false);
          }
        };
      });
      if (current !== session) return;
      if (session.stopRequested) {
        socket.send(
          JSON.stringify({
            type: "session.finish",
            event_id: crypto.randomUUID(),
          }),
        );
        updateState("finishing");
        audioCleanup(session);
        session.finishTimer = setTimeout(() => settle(session, false), 12000);
        return;
      }
      const context = new AudioContext({ sampleRate: 16000 });
      session.context = context;
      await context.resume();
      const moduleURL = URL.createObjectURL(
        new Blob([processorSource], { type: "application/javascript" }),
      );
      try {
        await context.audioWorklet.addModule(moduleURL);
      } finally {
        URL.revokeObjectURL(moduleURL);
      }
      if (current !== session) return;
      const node = new AudioWorkletNode(context, "voice-pcm");
      session.node = node;
      session.source = context.createMediaStreamSource(stream);
      const silent = context.createGain();
      silent.gain.value = 0;
      session.source.connect(node).connect(silent).connect(context.destination);
      node.port.onmessage = ({ data }) => {
        if (current !== session) return;
        if (
          data.pcm &&
          !session.audioClosed &&
          socket.readyState === WebSocket.OPEN
        ) {
          if (socket.bufferedAmount > 262144) {
            error("网络发送积压，已结束录音并保留文字。");
            void stop(false);
            return;
          }
          const bytes = new Uint8Array(data.pcm);
          let raw = "";
          for (let i = 0; i < bytes.length; i++)
            raw += String.fromCharCode(bytes[i]);
          socket.send(
            JSON.stringify({
              type: "input_audio_buffer.append",
              event_id: crypto.randomUUID(),
              audio: btoa(raw),
            }),
          );
        }
        if (data.flushed) session.flushResolve?.();
      };
      session.startedAt = Date.now();
      session.timer = setInterval(() => {
        const n = Math.floor((Date.now() - session.startedAt) / 1000);
        $("elapsed").textContent =
          String(Math.floor(n / 60)).padStart(2, "0") +
          ":" +
          String(n % 60).padStart(2, "0");
        if (n >= 280) void stop(false);
      }, 250);
      $("config").open = false;
      updateState("listening");
      if (session.stopRequested) void stop(false);
    } catch (e) {
      clearTimeout(session.startTimeout);
      if (current === session) {
        error(
          e.name === "NotAllowedError"
            ? "麦克风权限被拒绝，请允许录音后重试。"
            : e.message,
        );
        settle(session, true);
      }
    }
  }
  async function stop(cancelled = false) {
    const session = current;
    if (!session) return;
    if (cancelled) {
      session.rejectStart?.(Error("已取消"));
      clearTimeout(session.startTimeout);
      settle(session, true);
      return;
    }
    if (state === "starting") {
      session.stopRequested = true;
      return;
    }
    if (state === "finishing") return;
    updateState("finishing");
    if (session.node)
      await Promise.race([
        new Promise((resolve) => {
          session.flushResolve = resolve;
          session.node.port.postMessage("flush");
        }),
        new Promise((resolve) => setTimeout(resolve, 300)),
      ]);
    if (current !== session) return;
    session.audioClosed = true;
    audioCleanup(session);
    if (session.socket?.readyState === WebSocket.OPEN) {
      session.socket.send(
        JSON.stringify({
          type: "session.finish",
          event_id: crypto.randomUUID(),
        }),
      );
      session.finishTimer = setTimeout(() => {
        error("等待最终结果超时，已保留收到的文字。");
        settle(session, false);
      }, 12000);
    } else settle(session, false);
  }
  mic.addEventListener("pointerdown", (e) => e.preventDefault());
  mic.addEventListener("click", () =>
    state === "idle" ? void start("click") : void stop(false),
  );
  cancelButton.addEventListener("click", () => void stop(true));
  editor.addEventListener("input", () => {
    if (current && !current.anchor.isConnected) void stop(true);
    $("send").disabled = state !== "idle" || !editor.textContent.trim();
  });
  $("send").addEventListener("click", () => {
    if (state !== "idle" || !editor.textContent.trim()) return;
    const message = document.createElement("div");
    message.className = "message";
    message.textContent = editor.textContent.trim();
    $("messages").append(message);
    editor.replaceChildren();
    updateState("idle");
    editor.focus();
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && state !== "idle") {
      e.preventDefault();
      void stop(true);
      return;
    }
    if (
      e.code !== "AltRight" ||
      e.repeat ||
      e.ctrlKey ||
      e.metaKey ||
      e.isComposing ||
      e.target instanceof HTMLInputElement
    )
      return;
    e.preventDefault();
    if (hold) return;
    hold = true;
    if (state === "idle") void start("hold");
  });
  document.addEventListener("keyup", (e) => {
    if (e.code === "AltRight") {
      e.preventDefault();
      hold = false;
      if (current?.mode === "hold") void stop(false);
    }
  });
  window.addEventListener("blur", () => {
    hold = false;
    if (current) void stop(false);
  });
  document.addEventListener("visibilitychange", () => {
    if (document.hidden && current) void stop(false);
  });
  window.addEventListener("pagehide", () => {
    if (current) {
      audioCleanup(current);
      current.socket?.close();
    }
  });
  updateState("idle");
})();
