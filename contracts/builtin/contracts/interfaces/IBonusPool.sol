// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.4;

interface IBonusPool {
    // binding stake for bonus
    function bindingStake(address _addr, uint256 _deltaEth) external ;
    // unbind stake and get bonus
    function unbindStakeAndGetBonus(address _addr, address payable _recipient, uint256 _deltaEth) external;
}