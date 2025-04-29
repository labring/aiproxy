import React, { useEffect, useRef } from 'react';
import { cn } from '@/lib/utils';

interface ParticlesBackgroundProps {
  className?: string;
  particleColor?: string;
  particleSize?: number;
  particleCount?: number;
  speed?: number;
}

export function ParticlesBackground({
  className,
  particleColor = "rgba(26, 99, 212, 0.1)",
  particleSize = 6,
  particleCount = 25,
  speed = 1
}: ParticlesBackgroundProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    
    let animationFrameId: number;
    const particles: {
      x: number;
      y: number;
      size: number;
      baseSize: number;
      speedX: number;
      speedY: number;
      opacity: number;
      baseOpacity: number;
      rotation: number;
      rotationSpeed: number;
      pulseSpeed: number;
      pulseAmount: number;
      pulseOffset: number;
    }[] = [];
    
    // 设置canvas尺寸为窗口大小
    const resizeCanvas = () => {
      if (canvas && canvas.parentElement) {
        canvas.width = canvas.parentElement.offsetWidth;
        canvas.height = canvas.parentElement.offsetHeight;
        
        // 重新初始化粒子
        initParticles();
      }
    };
    
    // 初始化粒子
    const initParticles = () => {
      particles.length = 0;
      for (let i = 0; i < particleCount; i++) {
        const baseSize = Math.random() * particleSize + particleSize / 2;
        const baseOpacity = Math.random() * 0.4 + 0.2;
        
        particles.push({
          x: Math.random() * canvas.width,
          y: Math.random() * canvas.height,
          baseSize: baseSize,
          size: baseSize,
          speedX: (Math.random() - 0.5) * speed,
          speedY: (Math.random() - 0.5) * speed,
          baseOpacity: baseOpacity,
          opacity: baseOpacity,
          rotation: Math.random() * 360,
          rotationSpeed: (Math.random() - 0.5) * 0.5,
          pulseSpeed: Math.random() * 0.02 + 0.01,
          pulseAmount: Math.random() * 0.5 + 0.5,
          pulseOffset: Math.random() * Math.PI * 2 // 随机相位偏移
        });
      }
    };
    
    // 绘制粒子
    const drawParticles = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      
      const now = Date.now() / 1000; // 当前时间(秒)用于动画
      
      particles.forEach(particle => {
        // 呼吸效果 - 大小和透明度随时间变化
        const pulse = Math.sin(now * particle.pulseSpeed + particle.pulseOffset) * particle.pulseAmount;
        particle.size = particle.baseSize * (1 + pulse * 0.2);
        particle.opacity = particle.baseOpacity * (1 + pulse * 0.1);
        
        ctx.save();
        ctx.translate(particle.x + particle.size / 2, particle.y + particle.size / 2);
        ctx.rotate((particle.rotation * Math.PI) / 180);
        
        // 设置方块颜色和透明度
        ctx.fillStyle = particleColor.replace(/rgba?\(([^)]+)\)/, 
          (_, p) => `rgba(${p.split(',').slice(0, 3).join(',')}, ${particle.opacity})`);
        
        // 绘制方块
        ctx.fillRect(-particle.size / 2, -particle.size / 2, particle.size, particle.size);
        
        ctx.restore();
        
        // 更新粒子位置
        particle.x += particle.speedX;
        particle.y += particle.speedY;
        particle.rotation += particle.rotationSpeed;
        
        // 边界检测与循环
        if (particle.x < -particle.size) particle.x = canvas.width;
        if (particle.x > canvas.width) particle.x = -particle.size;
        if (particle.y < -particle.size) particle.y = canvas.height;
        if (particle.y > canvas.height) particle.y = -particle.size;
      });
      
      animationFrameId = requestAnimationFrame(drawParticles);
    };
    
    // 初始化和启动动画
    window.addEventListener('resize', resizeCanvas);
    resizeCanvas();
    drawParticles();
    
    // 清理
    return () => {
      window.removeEventListener('resize', resizeCanvas);
      cancelAnimationFrame(animationFrameId);
    };
  }, [particleColor, particleSize, particleCount, speed]);
  
  return (
    <canvas
      ref={canvasRef}
      className={cn("absolute inset-0 w-full h-full pointer-events-none z-0", className)}
    />
  );
} 