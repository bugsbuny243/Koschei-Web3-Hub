import React, { useEffect, useRef, useState } from 'react';
import { SafeAreaView, StyleSheet, Text, TouchableOpacity, View } from 'react-native';

const TITLE = __TITLE__;
const THEME = __THEME__;
const PLAYER = __PLAYER__;
const ENEMIES = __ENEMIES__;
const COLLECTIBLES = __COLLECTIBLES__;
const WIN_CONDITION = __WIN__;
const PROJECT_ID = __PROJECT_ID__;
const icon = (value, fallback) => Array.from(String(value || fallback)).slice(0, 2).join('');

export default function App() {
  const [lane, setLane] = useState(1);
  const [objects, setObjects] = useState([]);
  const [score, setScore] = useState(0);
  const [lives, setLives] = useState(3);
  const [running, setRunning] = useState(false);
  const [won, setWon] = useState(false);
  const nextId = useRef(1);

  useEffect(() => {
    if (!running) return undefined;
    const spawn = setInterval(() => {
      const harmful = Math.random() < 0.38;
      const pool = harmful ? ENEMIES : COLLECTIBLES;
      const label = pool[Math.floor(Math.random() * pool.length)];
      setObjects(items => [...items, {
        id: nextId.current++,
        lane: Math.floor(Math.random() * 3),
        y: 0,
        harmful,
        label: icon(label, harmful ? 'X' : '+')
      }].slice(-18));
    }, 700);
    return () => clearInterval(spawn);
  }, [running]);

  useEffect(() => {
    if (!running) return undefined;
    const tick = setInterval(() => {
      let damage = 0;
      let gained = 0;
      setObjects(items => items.reduce((next, item) => {
        const moved = { ...item, y: item.y + 8 };
        if (moved.y >= 82) {
          if (moved.lane === lane) {
            if (moved.harmful) damage += 1;
            else gained += 10;
          }
        } else next.push(moved);
        return next;
      }, []));
      if (damage) setLives(value => Math.max(0, value - damage));
      if (gained) setScore(value => value + gained);
    }, 160);
    return () => clearInterval(tick);
  }, [running, lane]);

  useEffect(() => {
    if (lives <= 0) setRunning(false);
    if (score >= 100) {
      setWon(true);
      setRunning(false);
    }
  }, [lives, score]);

  const start = () => {
    setLane(1);
    setObjects([]);
    setScore(0);
    setLives(3);
    setWon(false);
    setRunning(true);
  };

  return (
    <SafeAreaView style={styles.screen}>
      <View style={styles.header}>
        <Text style={styles.eyebrow}>KOSCHEI AUTO GAME · {PROJECT_ID.slice(0, 8)}</Text>
        <Text style={styles.title}>{TITLE}</Text>
        <Text style={styles.theme}>{THEME}</Text>
        <View style={styles.stats}>
          <Text style={styles.stat}>Score {score}</Text>
          <Text style={styles.stat}>Lives {lives}</Text>
        </View>
      </View>

      <View style={styles.arena}>
        {[0, 1, 2].map(value => <View key={value} style={[styles.lane, value === lane && styles.activeLane]} />)}
        {objects.map(item => <Text key={item.id} style={[styles.object, { left: `${item.lane * 33 + 12}%`, top: `${item.y}%` }]}>{item.label}</Text>)}
        <View style={[styles.player, { left: `${lane * 33 + 10}%` }]}>
          <Text style={styles.playerText}>{icon(PLAYER, 'P')}</Text>
        </View>
        {!running && <View style={styles.overlay}>
          <Text style={styles.overlayTitle}>{won ? 'YOU WIN' : lives <= 0 ? 'GAME OVER' : TITLE}</Text>
          <Text style={styles.overlayText}>{won ? WIN_CONDITION : 'Collect items, avoid enemies, reach 100 points.'}</Text>
          <TouchableOpacity style={styles.startButton} onPress={start}>
            <Text style={styles.startText}>{score || lives <= 0 ? 'RESTART' : 'START'}</Text>
          </TouchableOpacity>
        </View>}
      </View>

      <View style={styles.controls}>
        <TouchableOpacity style={styles.control} onPress={() => setLane(value => Math.max(0, value - 1))}><Text style={styles.controlText}>LEFT</Text></TouchableOpacity>
        <TouchableOpacity style={styles.control} onPress={() => setLane(value => Math.min(2, value + 1))}><Text style={styles.controlText}>RIGHT</Text></TouchableOpacity>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: '#02060b', padding: 18 },
  header: { paddingTop: 8, paddingBottom: 14 },
  eyebrow: { color: '#18ffb2', fontSize: 10, fontWeight: '800', letterSpacing: 1.3 },
  title: { color: '#f5fbff', fontSize: 30, fontWeight: '900', marginTop: 6 },
  theme: { color: '#8fa4b5', marginTop: 4 },
  stats: { flexDirection: 'row', gap: 12, marginTop: 12 },
  stat: { color: '#27dfff', fontWeight: '800' },
  arena: { flex: 1, position: 'relative', overflow: 'hidden', borderWidth: 1, borderColor: '#18ffb244', borderRadius: 24, backgroundColor: '#07121d' },
  lane: { position: 'absolute', top: 0, bottom: 0, width: '33.33%', borderRightWidth: 1, borderRightColor: '#ffffff12' },
  activeLane: { backgroundColor: '#18ffb20b' },
  object: { position: 'absolute', color: '#ffffff', fontSize: 24 },
  player: { position: 'absolute', bottom: 18, width: '14%', aspectRatio: 1, borderRadius: 18, alignItems: 'center', justifyContent: 'center', backgroundColor: '#18ffb2' },
  playerText: { color: '#02100d', fontSize: 22, fontWeight: '900' },
  overlay: { ...StyleSheet.absoluteFillObject, alignItems: 'center', justifyContent: 'center', padding: 24, backgroundColor: '#02060bdd' },
  overlayTitle: { color: '#f5fbff', fontSize: 28, fontWeight: '900', textAlign: 'center' },
  overlayText: { color: '#9db1bf', textAlign: 'center', marginTop: 10, lineHeight: 20 },
  startButton: { marginTop: 20, minWidth: 150, padding: 14, borderRadius: 16, backgroundColor: '#18ffb2' },
  startText: { color: '#02100d', textAlign: 'center', fontWeight: '900' },
  controls: { flexDirection: 'row', gap: 12, paddingTop: 14 },
  control: { flex: 1, padding: 16, borderRadius: 16, borderWidth: 1, borderColor: '#27dfff55', backgroundColor: '#0a1723' },
  controlText: { color: '#f5fbff', textAlign: 'center', fontWeight: '900' }
});
