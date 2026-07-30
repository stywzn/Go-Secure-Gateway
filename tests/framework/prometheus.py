"""极简 Prometheus 文本解析 —— 支撑"监控断言"。

只解析我们要断言的计数器/直方图样本，不引重依赖。
`/metrics` 暴露的关键指标（见 internal/metrics/metrics.go）：
  gateway_http_requests_total{method,route,status}
  gateway_http_request_duration_seconds_*{method,route}
"""
from __future__ import annotations

import re
from dataclasses import dataclass

_SAMPLE = re.compile(r'^(?P<name>[a-zA-Z_:][\w:]*)(?:\{(?P<labels>[^}]*)\})?\s+(?P<value>[^\s]+)')
_LABEL = re.compile(r'(\w+)="((?:[^"\\]|\\.)*)"')


@dataclass
class Sample:
    name: str
    labels: dict
    value: float


def parse(text: str) -> list[Sample]:
    samples: list[Sample] = []
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        m = _SAMPLE.match(line)
        if not m:
            continue
        labels = {k: v for k, v in _LABEL.findall(m.group("labels") or "")}
        try:
            value = float(m.group("value"))
        except ValueError:
            continue
        samples.append(Sample(m.group("name"), labels, value))
    return samples


def counter_value(text: str, name: str, **labels) -> float:
    """取匹配全部 label 的计数器值；找不到返回 0。"""
    for s in parse(text):
        if s.name == name and all(s.labels.get(k) == v for k, v in labels.items()):
            return s.value
    return 0.0
