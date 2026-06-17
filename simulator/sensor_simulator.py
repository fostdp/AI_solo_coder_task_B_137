#!/usr/bin/env python3
"""
三弓床弩传感器模拟器
模拟通过UDP上报弓弦拉力、弩臂变形、箭矢初速、穿甲深度。
支持拉力等级（light/mid/high/extreme）、箭镞类型、老化速度等场景配置。
"""

import socket
import json
import time
import random
import math
import argparse
import os
from datetime import datetime, timezone


TENSION_LEVELS = {
    "light":   {"tension": 2500.0, "velocity": 85.0,  "deform": 5.0,  "spin": 18.0, "label": "轻型拉力（训练用）"},
    "mid":     {"tension": 4500.0, "velocity": 120.0, "deform": 8.0,  "spin": 25.0, "label": "标准拉力（实战用）"},
    "high":    {"tension": 6500.0, "velocity": 150.0, "deform": 11.0, "spin": 32.0, "label": "重型拉力（攻城用）"},
    "extreme": {"tension": 8200.0, "velocity": 175.0, "deform": 14.0, "spin": 38.0, "label": "极限拉力（破甲用，有裂纹风险）"},
}

ARROW_HEADS = {
    "bodkin":     {"mass_ratio": 1.0,  "spin_factor": 1.0,  "label": "穿甲箭镞（bodkin）"},
    "broadhead":  {"mass_ratio": 1.15, "spin_factor": 0.85, "label": "宽刃箭镞（broadhead）"},
    "blunt":      {"mass_ratio": 1.3,  "spin_factor": 0.6,  "label": "钝头箭镞（blunt）"},
    "whistler":   {"mass_ratio": 0.95, "spin_factor": 0.9,  "label": "鸣笛箭镞（whistler，信号用）"},
}


class BallisticsSimulator:
    def __init__(self,
                 host="127.0.0.1",
                 port=8080,
                 device_id="chuangnu-001",
                 interval=60,
                 tension_level="mid",
                 arrow_head="bodkin",
                 wear_rate=0.0005,
                 ambient_temp=20.0,
                 ambient_humidity=50.0,
                 seed=None,
                 output_file=None):
        self.host = host
        self.port = port
        self.device_id = device_id
        self.interval = interval
        self.tension_level = tension_level
        self.arrow_head = arrow_head
        self.wear_rate = wear_rate
        self.ambient_temp = ambient_temp
        self.ambient_humidity = ambient_humidity
        self.output_file = output_file
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

        if seed is not None:
            random.seed(seed)

        lvl = TENSION_LEVELS[tension_level]
        ah  = ARROW_HEADS[arrow_head]
        self.base_tension    = lvl["tension"]
        self.base_deformation = lvl["deform"]
        self.base_velocity   = lvl["velocity"] / math.sqrt(ah["mass_ratio"])
        self.base_spin       = lvl["spin"] * ah["spin_factor"]
        self.base_penetration = 0.002 * (self.base_velocity / 120.0) ** 2

        self.shot_count = 0
        self._log_fp = None
        if output_file:
            self._log_fp = open(output_file, "a", encoding="utf-8")

    def summary(self):
        lvl = TENSION_LEVELS[self.tension_level]
        ah  = ARROW_HEADS[self.arrow_head]
        return [
            f"目标端点: {self.host}:{self.port} (UDP)",
            f"设备ID:   {self.device_id}",
            f"上报间隔: {self.interval}秒",
            f"拉力等级: {self.tension_level} — {lvl['label']}",
            f"箭镞类型: {self.arrow_head} — {ah['label']}",
            f"老化速度: wear_rate={self.wear_rate:.6f}/发",
            f"环境:     温度 {self.ambient_temp}°C / 湿度 {self.ambient_humidity}%",
            f"基准参数: T={self.base_tension:.0f}N, v={self.base_velocity:.1f}m/s, "
            f"deform={self.base_deformation:.1f}mm, spin={self.base_spin:.1f}Hz",
        ]

    def simulate_shot(self):
        self.shot_count += 1
        wear_factor = 1.0 + self.shot_count * self.wear_rate
        ah = ARROW_HEADS[self.arrow_head]

        tension = self.base_tension + random.gauss(0, self.base_tension * 0.06) * wear_factor
        deformation = self.base_deformation + abs(random.gauss(0, self.base_deformation * 0.18)) * wear_factor
        velocity = (self.base_velocity + random.gauss(0, self.base_velocity * 0.06)) / (1 + 0.05 * (wear_factor - 1))
        spin_rate = (self.base_spin + random.gauss(0, self.base_spin * 0.1)) * ah["spin_factor"]
        penetration = self.base_penetration * (velocity / self.base_velocity) ** 2 * (1 / ah["mass_ratio"])

        if self.shot_count % 50 == 0:
            deformation += random.uniform(self.base_deformation * 0.5, self.base_deformation * 1.2)
            velocity *= random.uniform(0.6, 0.82)

        if self.shot_count % 100 == 0:
            velocity *= random.uniform(0.48, 0.72)

        if deformation < 0:
            deformation = 0.1
        if velocity < 10:
            velocity = 10
        if penetration < 0:
            penetration = 0.0001
        if spin_rate < 0:
            spin_rate = 0.0

        temp = self.ambient_temp + random.gauss(0, 3.0)
        humidity = max(0, min(100, self.ambient_humidity + random.gauss(0, 10.0)))

        data = {
            "device_id": self.device_id,
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "bowstring_tension": round(tension, 2),
            "arm_deformation": round(deformation, 3),
            "arrow_initial_velocity": round(velocity, 2),
            "arrow_spin_rate": round(spin_rate, 2),
            "penetration_depth": round(penetration, 6),
            "temperature": round(temp, 1),
            "humidity": round(humidity, 1),
            "tension_level": self.tension_level,
            "arrow_head_type": self.arrow_head,
            "shot_index": self.shot_count,
        }
        return data

    def send_data(self, data):
        payload = json.dumps(data, ensure_ascii=False).encode("utf-8")
        try:
            self.sock.sendto(payload, (self.host, self.port))
            if self._log_fp:
                self._log_fp.write(payload.decode("utf-8") + "\n")
                self._log_fp.flush()
            return True
        except Exception as e:
            print(f"  ❌ 发送失败: {e}")
            return False

    def pretty_print(self, data, ok):
        status = "✅" if ok else "❌"
        wear_pct = self.shot_count * self.wear_rate * 100
        print(f"[{data['timestamp']}] {status} #{self.shot_count:>6}  "
              f"T={data['bowstring_tension']:>7.0f}N  "
              f"δ={data['arm_deformation']:>6.2f}mm  "
              f"v₀={data['arrow_initial_velocity']:>6.1f}m/s  "
              f"f={data['arrow_spin_rate']:>5.1f}Hz  "
              f"D={data['penetration_depth']*1000:>6.2f}mm  "
              f"(wear={wear_pct:.2f}%)")

    def run(self, count=0, once=False):
        for line in self.summary():
            print(f"  {line}")
        print("=" * 78)

        if once:
            d = self.simulate_shot()
            ok = self.send_data(d)
            self.pretty_print(d, ok)
            self.close()
            return

        loop_total = count if count > 0 else None
        try:
            while loop_total is None or self.shot_count < loop_total:
                d = self.simulate_shot()
                ok = self.send_data(d)
                self.pretty_print(d, ok)
                time.sleep(self.interval)
        except KeyboardInterrupt:
            print("\n🛑 模拟器已停止（Ctrl+C）")
        finally:
            self.close()

    def close(self):
        if self._log_fp:
            self._log_fp.close()
        self.sock.close()


def env_or(key, default, conv=str):
    v = os.environ.get(key)
    return default if v is None or v == "" else conv(v)


def main():
    parser = argparse.ArgumentParser(
        description="三弓床弩传感器模拟器",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
拉力等级:
  light    训练用   ~2500N / 85m/s
  mid      实战用   ~4500N / 120m/s (默认)
  high     攻城用   ~6500N / 150m/s
  extreme  破甲用   ~8200N / 175m/s (有裂纹风险)

箭镞类型:
  bodkin     穿甲箭镞 (默认)
  broadhead  宽刃箭镞 (反人员)
  blunt      钝头箭镞 (眩晕/破盾)
  whistler   鸣笛箭镞 (信号/恐吓)

示例:
  # 标准拉力、穿甲箭镞，1分钟一发
  python sensor_simulator.py

  # 极限拉力、宽刃镞，5秒一发，打100发
  python sensor_simulator.py --level extreme --arrow broadhead -i 5 -n 100

  # 3台设备不同拉力等级同时模拟（配 docker-compose 推荐）
  python sensor_simulator.py --device chuangnu-001 --level light   &
  python sensor_simulator.py --device chuangnu-002 --level mid     &
  python sensor_simulator.py --device chuangnu-003 --level extreme &
        """,
    )
    parser.add_argument("--host",       default=env_or("SIM_HOST", "127.0.0.1"), help="UDP目标主机")
    parser.add_argument("--port",       type=int, default=env_or("SIM_PORT", 8080, int), help="UDP目标端口")
    parser.add_argument("--device",     default=env_or("SIM_DEVICE", "chuangnu-001"), help="设备ID")
    parser.add_argument("-i", "--interval", type=int, default=env_or("SIM_INTERVAL", 60, int), help="上报间隔(秒)")
    parser.add_argument("-l", "--level",    default=env_or("SIM_LEVEL", "mid"), choices=TENSION_LEVELS.keys(), help="拉力等级")
    parser.add_argument("-a", "--arrow",    default=env_or("SIM_ARROW", "bodkin"), choices=ARROW_HEADS.keys(), help="箭镞类型")
    parser.add_argument("-w", "--wear",     type=float, default=env_or("SIM_WEAR", 0.0005, float), help="每发射老化系数 (推荐 1e-5 ~ 5e-3)")
    parser.add_argument("--temp",       type=float, default=env_or("SIM_TEMP", 20.0, float), help="环境温度 °C")
    parser.add_argument("--humid",      type=float, default=env_or("SIM_HUMID", 50.0, float), help="环境湿度 %")
    parser.add_argument("--seed",       type=int, default=env_or("SIM_SEED", 0, int), help="随机种子（0=不固定）")
    parser.add_argument("--log",        default=env_or("SIM_LOG", ""), help="将发送的JSON逐行写入文件")
    parser.add_argument("--once",       action="store_true", help="只发送一次后退出")
    parser.add_argument("-n", "--count",   type=int, default=env_or("SIM_COUNT", 0, int), help="发送指定次数后退出（0=无限）")
    parser.add_argument("--list-levels", action="store_true", help="打印拉力等级表后退出")
    parser.add_argument("--list-arrows", action="store_true", help="打印箭镞类型表后退出")

    args = parser.parse_args()

    if args.list_levels:
        print("拉力等级表:")
        for k, v in TENSION_LEVELS.items():
            print(f"  {k:>8s}:  T={v['tension']:>6.0f}N  v₀={v['velocity']:>6.1f}m/s  "
                  f"δ={v['deform']:>4.1f}mm  f={v['spin']:>4.1f}Hz   — {v['label']}")
        return

    if args.list_arrows:
        print("箭镞类型表:")
        for k, v in ARROW_HEADS.items():
            print(f"  {k:>10s}:  质量x{v['mass_ratio']:.2f}  自旋x{v['spin_factor']:.2f}   — {v['label']}")
        return

    seed = None if args.seed == 0 else args.seed
    sim = BallisticsSimulator(
        host=args.host,
        port=args.port,
        device_id=args.device,
        interval=args.interval,
        tension_level=args.level,
        arrow_head=args.arrow,
        wear_rate=args.wear,
        ambient_temp=args.temp,
        ambient_humidity=args.humid,
        seed=seed,
        output_file=args.log if args.log else None,
    )
    sim.run(count=args.count, once=args.once)


if __name__ == "__main__":
    main()
