// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

#include "textflag.h"

TEXT ·CPUInit(SB),NOSPLIT|NOFRAME,$0
	JMP	cpuinit(SB)
