#!/usr/bin/env python3
"""Exercise real terminal processes with a PTY, including Kitty negotiation."""
import os, pty, select, struct, subprocess, termios, time, fcntl
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

def run(kitty=False):
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 32, 120, 1200, 640))
    env = dict(os.environ, TERM='xterm-256color', COLORTERM='truecolor')
    env.pop('TMUX', None)
    proc = subprocess.Popen([str(ROOT / 'bin/hilife'), '--pattern', 'glider'], stdin=slave, stdout=slave, stderr=slave, env=env)
    os.close(slave)
    output = bytearray()
    answered=False
    quit_at=time.monotonic()+2
    deadline=time.monotonic()+8
    try:
        while time.monotonic()<deadline:
            if select.select([master], [], [], .05)[0]:
                try: chunk=os.read(master, 65536)
                except OSError: break
                if not chunk: break
                output.extend(chunk)
                if kitty and not answered and b'\x1b_Gi=31' in output:
                    os.write(master,b'\x1b_Gi=31;OK\x1b\\\x1b[6;20;10t')
                    answered=True
            if time.monotonic()>quit_at and proc.poll() is None:
                os.write(master,b'q')
                quit_at=deadline+1
            if proc.poll() is not None: break
        proc.wait(timeout=2)
        assert proc.returncode==0, bytes(output[-2000:])
        if kitty:
            assert b'a=T,t=d,f=100' in output, 'Kitty image was not transmitted'
            assert b'a=d,d=I,i=4242' in output, 'Kitty image cleanup missing'
        else:
            text=output.decode('utf-8','ignore')
            assert any(0x2801<=ord(c)<=0x28ff for c in text), 'Braille fallback missing'
            assert b'a=T,t=d,f=100' not in output, 'Image sent without a positive capability response'
        print(('Kitty handshake, PNG transfer and cleanup' if kitty else 'Automatic Braille fallback')+': PASS')
    finally:
        if proc.poll() is None: proc.kill(); proc.wait()
        os.close(master)

run(False)
run(True)
