import { useEffect, useRef } from 'react';
import { Link } from 'expo-router';
import {
  Animated,
  Easing,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

const CYAN = '#00e5ff';
const VIOLET = '#9d4edd';
const MAGENTA = '#ff2bd1';
const BG = '#04020a';

/* ---- Matrix rain column ---- */
function RainColumn({ left, delay, duration }: { left: number; delay: number; duration: number }) {
  const y = useRef(new Animated.Value(-200)).current;
  useEffect(() => {
    const loop = Animated.loop(
      Animated.timing(y, {
        toValue: 900,
        duration,
        delay,
        easing: Easing.linear,
        useNativeDriver: true,
      }),
    );
    loop.start();
    return () => loop.stop();
  }, [y, delay, duration]);

  const chars = 'カキクケコ01KOSCHEIΦΨ'.split('');
  return (
    <Animated.View style={[styles.rainCol, { left, transform: [{ translateY: y }] }]}>
      {Array.from({ length: 14 }).map((_, i) => (
        <Text
          key={i}
          style={{
            color: i === 0 ? CYAN : i < 4 ? VIOLET : 'rgba(157,78,221,0.35)',
            fontSize: 13,
            fontFamily: 'monospace',
            lineHeight: 16,
          }}
        >
          {chars[(i + left) % chars.length]}
        </Text>
      ))}
    </Animated.View>
  );
}

/* ---- Pulsing glow button ---- */
function GlowButton({ label, href, primary }: { label: string; href: string; primary?: boolean }) {
  const glow = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    if (!primary) return;
    Animated.loop(
      Animated.sequence([
        Animated.timing(glow, { toValue: 1, duration: 1400, useNativeDriver: false }),
        Animated.timing(glow, { toValue: 0, duration: 1400, useNativeDriver: false }),
      ]),
    ).start();
  }, [glow, primary]);

  const shadow = glow.interpolate({ inputRange: [0, 1], outputRange: [10, 28] });

  return (
    <Link href={href} asChild>
      <Pressable>
        <Animated.View
          style={[
            styles.btn,
            primary ? styles.btnPrimary : styles.btnGhost,
            primary ? { shadowRadius: shadow as any } : null,
          ]}
        >
          <Text style={[styles.btnText, { color: primary ? BG : CYAN }]}>{label}</Text>
        </Animated.View>
      </Pressable>
    </Link>
  );
}

/* ---- Rotating reactor ring ---- */
function Ring({ size, color, duration, reverse }: { size: number; color: string; duration: number; reverse?: boolean }) {
  const spin = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    Animated.loop(
      Animated.timing(spin, { toValue: 1, duration, easing: Easing.linear, useNativeDriver: true }),
    ).start();
  }, [spin, duration]);
  const rotate = spin.interpolate({
    inputRange: [0, 1],
    outputRange: reverse ? ['360deg', '0deg'] : ['0deg', '360deg'],
  });
  return (
    <Animated.View
      style={{
        position: 'absolute',
        width: size,
        height: size,
        borderRadius: size / 2,
        borderWidth: 1,
        borderColor: 'rgba(0,229,255,0.18)',
        borderTopColor: color,
        transform: [{ rotate }],
      }}
    />
  );
}

const MODULES = [
  { ico: '⌬', name: 'CODE', desc: 'Üretim hazır kod' },
  { ico: '◈', name: 'IMAGE', desc: 'Sinematik görsel' },
  { ico: '▣', name: 'VIDEO', desc: 'Hareketli sahne' },
  { ico: '◊', name: 'AUDIO', desc: 'Doğal seslendirme' },
  { ico: '⬢', name: 'CHAT', desc: 'Akıllı diyalog' },
  { ico: '⟁', name: 'REASON', desc: 'Derin analiz' },
];

export default function Home() {
  const corePulse = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    Animated.loop(
      Animated.sequence([
        Animated.timing(corePulse, { toValue: 1, duration: 1200, useNativeDriver: true }),
        Animated.timing(corePulse, { toValue: 0, duration: 1200, useNativeDriver: true }),
      ]),
    ).start();
  }, [corePulse]);
  const coreScale = corePulse.interpolate({ inputRange: [0, 1], outputRange: [1, 1.12] });

  return (
    <View style={styles.root}>
      {/* matrix rain */}
      <View style={styles.rainLayer} pointerEvents="none">
        {Array.from({ length: 12 }).map((_, i) => (
          <RainColumn
            key={i}
            left={i * 34 + 6}
            delay={i * 260}
            duration={5200 + (i % 4) * 1100}
          />
        ))}
      </View>

      <ScrollView contentContainerStyle={styles.scroll}>
        {/* status bar */}
        <View style={styles.statusBar}>
          <View style={{ flexDirection: 'row', alignItems: 'center' }}>
            <View style={styles.dot} />
            <Text style={styles.statusText}>SYSTEM ONLINE</Text>
          </View>
          <Text style={styles.statusText}>NODE: EU-CENTRAL-1</Text>
        </View>

        {/* title */}
        <Text style={styles.subtitle}>// QUANTUM AI COMMAND CENTER</Text>
        <Text style={styles.title}>KOSCHEI</Text>
        <Text style={[styles.subtitle, { color: VIOLET }]}>THE IMMORTAL AI</Text>

        <Text style={styles.tagline}>
          9 yapay zeka modeli. Tek komuta merkezi. Türkçe fikrini söyle —{' '}
          <Text style={{ color: VIOLET }}>kod, görsel, video, ses</Text> olarak geri al.
        </Text>

        {/* buttons */}
        <View style={{ gap: 14, marginBottom: 36 }}>
          <GlowButton label="▶ SİSTEME GİRİŞ — ÜCRETSİZ" href="/register" primary />
          <GlowButton label="⬡ MEVCUT KULLANICI / LOGIN" href="/login" />
        </View>

        {/* modules */}
        <View style={styles.sectLabel}>
          <View style={styles.sectLine} />
          <Text style={styles.sectText}>AKTİF MODÜLLER</Text>
        </View>
        <View style={styles.modGrid}>
          {MODULES.map((m, i) => (
            <View
              key={m.name}
              style={[
                styles.mod,
                { borderColor: i % 2 ? 'rgba(157,78,221,0.3)' : 'rgba(0,229,255,0.3)' },
              ]}
            >
              <Text style={styles.modIco}>{m.ico}</Text>
              <Text style={styles.modName}>{m.name}</Text>
              <Text style={styles.modDesc}>{m.desc}</Text>
            </View>
          ))}
        </View>

        {/* quantum core */}
        <View style={styles.sectLabel}>
          <View style={styles.sectLine} />
          <Text style={styles.sectText}>QUANTUM ÇEKİRDEK</Text>
        </View>
        <View style={styles.reactor}>
          <Ring size={200} color={CYAN} duration={6000} />
          <Ring size={150} color={VIOLET} duration={4000} reverse />
          <Ring size={100} color={MAGENTA} duration={3000} />
          <Animated.View style={[styles.core, { transform: [{ scale: coreScale }] }]}>
            <Text style={styles.coreNum}>99.9%</Text>
            <Text style={styles.coreLbl}>ÇEKİRDEK STABİL</Text>
          </Animated.View>
        </View>

        <Text style={styles.foot}>KOSCHEI RUNTIME v1.0 — TRADEPIGLOBALL.CO</Text>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: BG },
  rainLayer: { position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, opacity: 0.22 },
  rainCol: { position: 'absolute', top: 0 },
  scroll: { padding: 20, paddingTop: 54, paddingBottom: 60 },

  statusBar: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    borderWidth: 1,
    borderColor: 'rgba(0,229,255,0.25)',
    backgroundColor: 'rgba(0,229,255,0.04)',
    borderRadius: 8,
    paddingVertical: 8,
    paddingHorizontal: 12,
    marginBottom: 30,
  },
  dot: {
    width: 7,
    height: 7,
    borderRadius: 4,
    backgroundColor: '#00ff9d',
    marginRight: 6,
  },
  statusText: { color: CYAN, fontSize: 10, letterSpacing: 2, fontFamily: 'monospace' },

  subtitle: { color: CYAN, fontSize: 12, letterSpacing: 3, fontFamily: 'monospace' },
  title: {
    color: '#fff',
    fontSize: 56,
    fontWeight: '900',
    letterSpacing: 2,
    marginVertical: 4,
    textShadowColor: 'rgba(0,229,255,0.7)',
    textShadowRadius: 18,
  },
  tagline: {
    color: '#7fa8c9',
    fontSize: 13,
    lineHeight: 21,
    marginTop: 18,
    marginBottom: 30,
    fontFamily: 'monospace',
  },

  btn: {
    paddingVertical: 17,
    borderRadius: 10,
    alignItems: 'center',
    shadowColor: CYAN,
    shadowOpacity: 0.6,
    shadowOffset: { width: 0, height: 0 },
  },
  btnPrimary: { backgroundColor: CYAN },
  btnGhost: {
    borderWidth: 1,
    borderColor: 'rgba(0,229,255,0.4)',
    backgroundColor: 'rgba(0,229,255,0.05)',
  },
  btnText: { fontSize: 14, fontWeight: '700', letterSpacing: 2 },

  sectLabel: { flexDirection: 'row', alignItems: 'center', marginBottom: 14, gap: 8 },
  sectLine: { width: 22, height: 1, backgroundColor: VIOLET },
  sectText: { color: VIOLET, fontSize: 10, letterSpacing: 3, fontFamily: 'monospace' },

  modGrid: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'space-between', marginBottom: 32 },
  mod: {
    width: '48%',
    borderWidth: 1,
    borderRadius: 10,
    padding: 14,
    marginBottom: 12,
    backgroundColor: 'rgba(0,229,255,0.04)',
  },
  modIco: { fontSize: 22, marginBottom: 8, color: '#fff' },
  modName: { color: '#fff', fontWeight: '700', fontSize: 14, letterSpacing: 1 },
  modDesc: { color: '#6f93b3', fontSize: 10, marginTop: 3, fontFamily: 'monospace' },

  reactor: {
    height: 220,
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: 18,
  },
  core: {
    width: 88,
    height: 88,
    borderRadius: 44,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: 'rgba(0,229,255,0.12)',
    borderWidth: 1,
    borderColor: 'rgba(0,229,255,0.4)',
    shadowColor: CYAN,
    shadowOpacity: 0.7,
    shadowRadius: 30,
    shadowOffset: { width: 0, height: 0 },
  },
  coreNum: { color: '#fff', fontSize: 18, fontWeight: '900' },
  coreLbl: { color: CYAN, fontSize: 7, letterSpacing: 1.5, marginTop: 3 },

  foot: {
    color: '#3d5f7a',
    fontSize: 9,
    letterSpacing: 2,
    textAlign: 'center',
    marginTop: 34,
    fontFamily: 'monospace',
  },
});
