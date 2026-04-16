# Description:
#   wave.py - Software-define sensors for wave measurements.
#
#   See also http://netreceiver.appspot.com
#
# Author:
#   Emily Braggs <emily@ausocean.org>
#
# License:
#   Copyright (C) 2019 The Australian Ocean Lab (AusOcean)
#
#   This file is part of NetReceiver. NetReceiver is free software: you can
#   redistribute it and/or modify it under the terms of the GNU
#   General Public License as published by the Free Software
#   Foundation, either version 3 of the License, or (at your option)
#   any later version.
#
#   NetReceiver is distributed in the hope that it will be useful,
#   but WITHOUT ANY WARRANTY; without even the implied warranty of
#   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#   GNU General Public License for more details.
#
#   You should have received a copy of the GNU General Public License
#   along with NetReceiver in gpl.txt.  If not, see
#   <http://www.gnu.org/licenses/>.
#

import struct
import math
from operator import mul
import logging

GRAVITY = 9.81 #ms^-2
MIN_ACCELERATION = 0.4 #ms^-2 (empirical)
MIN_WAVELENGTH = 5 #m
MAX_WAVELENGTH = 300 #m
MIN_ACCURACY = 5 #m

def matrixMul(matrix1, matrix2):
  """Takes input of two list of lists and performs matrix multiplication on them returning the resulting list of lists."""
  return [[sum(map(mul, row, col)) for col in zip(*matrix2)] for row in matrix1]


def eulerAnglesToRotationMatrix(eul):
  """Takes input of a eulers angles in a list then returns their corresponding rotation matrix as a list."""
  rotm = [[0,0,0],[0,0,0],[0,0,0]]

  s1 = math.sin(eul[0])
  c1 = math.cos(eul[0])
  s2 = math.sin(eul[1])
  c2 = math.cos(eul[1])
  s3 = math.sin(eul[2])
  c3 = math.cos(eul[2])

  rotm[0][0] = c1*c2
  rotm[0][1] = c1*s2*s3 - s1*c3
  rotm[0][2] = c1*s2*c3 + s1*s3

  rotm[1][0] = s1*c2
  rotm[1][1] = s1*s2*s3 + c1*c3
  rotm[1][2] = s1*s2*c3 - c1*s3

  rotm[2][0] = -s2
  rotm[2][1] = c2*s3
  rotm[2][2] = c2*c3

  return rotm

def transpose(self):
  """transposes a list of lists."""
  return list(map(list, zip(self)))

def removeGravity(rawSamples, dec):
  """removeGravity takes input of list of lists each with 6 values in it. The first three are the x, y and z
  accelerometer readings and the final three are the x, y and z magnetometer readings. It rotates the acceleration
  component so that it is referenced to the earth not the rig, gravity component of acceleration is then removed."""
  rawSamples = list(rawSamples)
  leng = len(rawSamples)
  rolls = [0.0] * leng
  pitches = [0.0] * leng
  headings = [0.0] * leng
  gravity = [0,0,1]

  #extract acceleration samples from all samples
  accelerations = [sublist[:3] for sublist in rawSamples]
  accelerations = list(accelerations)
  #calculate roll, pitch and heading for each set of values
  for n in range(leng):
    rolls[n] = math.atan2(rawSamples[n][1], rawSamples[n][2])
    pitches[n] = math.atan2(-rawSamples[n][0], math.sqrt(rawSamples[n][1]*rawSamples[n][1] + rawSamples[n][2]*rawSamples[n][2]))
    headings[n] = math.atan2(rawSamples[n][3], rawSamples[n][4])
    headings[n] = headings[n] + dec*math.pi/180
    if headings[n] > math.pi:
      headings[n] = headings[n] - (2*math.pi)
    elif headings[n] < -math.pi:
      headings[n] = headings[n] + (2*math.pi)
    eul = [-headings[n], -pitches[n], -rolls[n]]
    rotationMat = eulerAnglesToRotationMatrix(eul)

    #rotate acceleration vectors
    accelerations[n] = matrixMul(rotationMat, transpose(accelerations[n][:]))

    #remove gravity component and offsets
    accelerations[n][0] = accelerations[n][0][0] * GRAVITY
    accelerations[n][1] = accelerations[n][1][0] * GRAVITY
    accelerations[n][2] = (accelerations[n][2][0] - 1) * GRAVITY
  return accelerations

def computePeriod(samples, samplePeriod):
  """Compute the period of the one dimensional list samples which holds the z accelerations"""
  smoothSamples = list(samples)
  smoothSamples = movingAverage(smoothSamples, int(1/(samplePeriod*0.001)))
  crossings = list()

  #find where smooth samples crosses the 0 mark
  for n in range(len(smoothSamples) - 1):
    if smoothSamples[n] < 0 and smoothSamples[n+1] > 0:
      crossings.append(n)
    elif smoothSamples[n] > 0 and smoothSamples[n+1] < 0:
      crossings.append(n)

  #calculate average period
  if len(crossings) < 2:
    logging.warn("wave height: Wave period greater than 20 sec. Could not compute period.")
    return None
  total = 0
  for n in range(len(crossings) - 1):
    period = crossings[n+1] - crossings[n]
    total += period
  period = (total/(len(crossings) - 1)) * 2
  period = samplePeriod*0.001*period
  print(period)
  return period

def computeDisplacement(accelerations, samplePeriod):
  """Compute displacement from inputted acceleration samples (list of lists with 3 accelerometer values in each list)."""
  zAccelerations = [0.0] * len(accelerations)
  zDisplacements = [0.0] * len(zAccelerations)

  #extract z acceleration samples from all acceleration samples
  for n in range(len(accelerations)):
    zAccelerations[n] = accelerations[n][2]

  #apply a 10 point moving average
  zAccelerations = movingAverage(zAccelerations, float(0.25/(samplePeriod*0.001)))

  #iterates through all z accelerations to check if there is significant acceleration
  zeroAccelCounter = 0
  for n in range(len(zAccelerations)):
    if zAccelerations[n] < MIN_ACCELERATION:
      zeroAccelCounter = zeroAccelCounter + 1

  #if there is no significant acceleration returns displacements of zero
  if zeroAccelCounter > len(zAccelerations) - 1:
    return zDisplacements

  period = computePeriod(zAccelerations, samplePeriod)
  if not period:
    return None

  #calculate zDisplacements
  for n in range(len(zAccelerations)):
    zDisplacements[n] = zAccelerations[n]/(-(2 * math.pi * (1/period)) * (2 * math.pi * (1/period)))
  return zDisplacements

def movingAverage(samples,k):
  """Applies a k-point moving average to the inputed one dimensional list."""
  k = int(k)
  for n in range(len(samples)):
    if (n > k/2 and n < len(samples) - k/2):
      samples[n] = sum(samples[n-k/2:n+k/2])/len(samples[n-k/2:n+k/2])
    elif (n < k/2):
      samples[n] = sum(samples[0:(n+k/2)])/len(samples[0:(n+k/2)])
    elif (n > len(samples[:]) - k/2):
      samples[n] = sum(samples[(n-k/2):])/len(samples[(n-k/2):])
  return samples

def waveHeight(data, dec):
  """waveHeight extracts degree-of-freedom binary data and computes displacement difference."""
  timestamp,samplePeriod, nSamples, buff = struct.unpack("IIII", data[:16]) 
  sz = len(data[16:])/4 # 4 bytes per float
  sz /= 6
  if sz != nSamples:
    logging.warn("wave height: size of data does not match specified size.")
    return -1
  rawSamples = [0.0] * sz
  for ii in range(sz):
    rawSamples[ii] = struct.unpack("ffffff", data[ii*24+16: ii*24+40])
  zDisplacements = computeDisplacement(removeGravity(rawSamples, dec), samplePeriod)
  if not zDisplacements:
    return -1
  return max(zDisplacements) - min(zDisplacements)

def wavePeriod(data, dec):
  """wavePeriod extracts degree-of-freedom binary data and computes the average period."""
  timestamp,samplePeriod, nSamples, buff = struct.unpack("IIII", data[0:16]) 
  sz = len(data[16:])/4 # 4 bytes per float
  sz /= 6
  if sz != nSamples:
    logging.warn("wave period: size of data does not match specified size.")
    return -1
  rawSamples = [0.0] * sz
  for ii in range(sz):
    rawSamples[ii] = struct.unpack("ffffff", data[ii*24+16: ii*24+40])
  accelerations = removeGravity(rawSamples, dec)
  zAccelerations = [0.0] * len(accelerations)

  #extract z acceleration samples from all acceleration samples
  for n in range(len(accelerations)):
    zAccelerations[n] = accelerations[n][2]
  period = computePeriod(zAccelerations, samplePeriod)
  if not period:
    return -1
  return period

def waveLength(data, depth, dec):
  timestamp,samplePeriod, nSamples, buff = struct.unpack("IIII", data[0:16]) 
  sz = len(data[16:])/4 # 4 bytes per float
  sz /= 6
  if sz != nSamples:
    logging.warn("wave length: size of data does not match specified size.")
    return -1
  rawSamples = [0.0] * sz
  for ii in range(sz):
    rawSamples[ii] = struct.unpack("ffffff", data[ii*24+16: ii*24+40])
  accelerations = removeGravity(rawSamples, dec)
  zAccelerations = [0.0] * len(accelerations)

  #extract z acceleration samples from all acceleration samples
  for n in range(len(accelerations)):
    zAccelerations[n] = accelerations[n][2]
  period = computePeriod(zAccelerations, samplePeriod)
  if not period:
    return -1
  if depth == 0:
    waveLen = computeDeepWaterWavelength(period)
  else:
    waveLen = computeWavelength(depth, period)
  return waveLen

def computeDeepWaterWavelength(period):
  """computes the wavelength assuming it is a deep water wave (depth greater than one half of the wavelength)"""
  length = (GRAVITY / (2 * math.pi)) * period * period
  return length

def computeWavelength(depth, period):
  """Computes the wavelength of the wave"""
  prevDiff = MIN_ACCURACY
  length = -1
  for n in range(MIN_WAVELENGTH, MAX_WAVELENGTH):
    current = ((GRAVITY*period*period)/(2*math.pi)) * math.tanh((2*math.pi*depth)/n)
    if math.fabs(current - n) < prevDiff:
      prevDiff = math.fabs(current - n)
      length = n
  if length == -1:
    logging.warn("wave length: wavelength could not be calculated as it was not within expected range.")
  return length
